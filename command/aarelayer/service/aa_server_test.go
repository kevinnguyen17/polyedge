package service

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/0xPolygon/polygon-edge/types"
	"github.com/btcsuite/btcd/btcec"
	"github.com/stretchr/testify/require"
)

const baseURL = "127.0.0.1:8289"

func Test_AAServer(t *testing.T) {
	t.Parallel()

	aaServer := getServer(t)

	go func() {
		aaServer.ListenAndServe(baseURL)
	}()

	t.Cleanup(func() {
		aaServer.Shutdown(context.Background())
	})

	priv, err := ecdsa.GenerateKey(btcec.S256(), rand.Reader)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 100) // wait for server to start

	t.Run("sendTransaction_getTransactionReceipt_ok", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{}
		tx := &AATransaction{
			Signature: nil,
			Transaction: Transaction{
				Nonce: 0,
				Payload: []Payload{
					{
						To: &types.Address{1, 2, 3},
					},
					{
						To:    nil,
						Value: 100,
					},
				},
			},
		}

		tx.Transaction.UpdateFrom(priv)

		require.NoError(t, tx.MakeSignature(priv))

		require.True(t, tx.IsFromValid())

		req := makeRequest(t, "POST", "sendTransaction", tx)

		res, err := client.Do(req)
		require.NoError(t, err)

		// Check that the response code is 200 OK
		require.Equal(t, http.StatusOK, res.StatusCode)

		// Check that the response body contains the expected data
		uuidBytes, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		var responseObj map[string]string

		require.NoError(t, json.Unmarshal(uuidBytes, &responseObj))

		uuid := responseObj["uuid"]

		require.True(t, len(uuid) > 10)

		req = makeRequest(t, "GET", fmt.Sprintf("getTransactionReceipt/%s", uuid), nil)

		res, err = client.Do(req)
		require.NoError(t, err)

		// Check that the response code is 200 OK
		require.Equal(t, http.StatusOK, res.StatusCode)

		// Check that the response body contains the expected data
		bytes, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		receipt := AAReceipt{}

		require.NoError(t, json.Unmarshal(bytes, &receipt))

		require.Equal(t, uuid, receipt.ID)
	})

	t.Run("sendTransaction_WrongMethod", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{}
		req := makeRequest(t, "GET", "sendTransaction", &AATransaction{})

		res, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
	})

	t.Run("getTransactionReceipt_WrongMethod", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{}
		req := makeRequest(t, "POST", "getTransactionReceipt/321", nil)

		res, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
	})

	t.Run("getTransactionReceipt_TxUUIDNotExist", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{}
		req := makeRequest(t, "GET", "getTransactionReceipt/321", nil)

		res, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})

	t.Run("sendTransaction_WrongInput", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{}

		req := makeRequest(t, "POST", "sendTransaction", nil)

		res, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("sendTransaction_EmptyPayload", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{}

		req := makeRequest(t, "POST", "sendTransaction", &AATransaction{})

		res, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("sendTransaction_WrongFrom", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{}
		tx := &AATransaction{
			Signature: nil,
			Transaction: Transaction{
				Nonce: 0,
				Payload: []Payload{
					{
						To: &types.Address{1, 2, 3},
					},
				},
			},
		}

		require.NoError(t, tx.MakeSignature(priv))

		req := makeRequest(t, "POST", "sendTransaction", &tx)

		res, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
}

func getServer(t *testing.T) *AARelayerRestServer {
	t.Helper()

	state, err := NewAATxState()
	require.NoError(t, err)

	config, err := GetConfig()
	require.NoError(t, err)

	pool := NewAAPool()
	verification := NewAAVerification(config, func(a *AATransaction) bool {
		return true
	})

	return NewAARelayerRestServer(pool, state, verification)
}

func makeRequest(t *testing.T, httpMethod, endpoint string, obj interface{}) *http.Request {
	t.Helper()

	var body io.Reader

	if obj != nil {
		var buf bytes.Buffer

		require.NoError(t, json.NewEncoder(&buf).Encode(obj))

		body = &buf
	}

	// Create a new request to the endpoint
	req, err := http.NewRequest(httpMethod, fmt.Sprintf("http://%s/v1/%s", baseURL, endpoint), body)
	require.NoError(t, err)

	return req
}