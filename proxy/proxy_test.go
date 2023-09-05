package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const PROXY_PORT = ":3333"

func setupProxyServer() *Proxy {
	proxy := Proxy{}
	go proxy.Start(PROXY_PORT)
	return &proxy
}

func setupHttpMockServer(handler http.Handler) *httptest.Server {
	if handler == nil {
		return httptest.NewServer(http.HandlerFunc(defaultMockServerResponse))
	}
	return httptest.NewServer(handler)
}

func BuildProxyTransport() http.Transport {
	proxy_url, _ := url.Parse("http://localhost" + PROXY_PORT)
	return http.Transport{
		Proxy: http.ProxyURL(proxy_url),
	}
}

func GetRequestTroughProxy(server_addr string, path string) (*http.Response, error) {
	client := BuildHttpClientWithProxy()
	return client.Get(server_addr + path)
}

func BuildHttpClientWithProxy() http.Client {
	transport := BuildProxyTransport()
	client := http.Client{
		Transport: &transport,
	}
	return client
}

func defaultMockServerResponse(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("hello world"))
}

func Test_MockServer(t *testing.T) {
	server := setupHttpMockServer(nil)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	assert.Equal(t, string(body), "hello world")
}

func Test_RequestThroughProxyAndMockServerNotUp(t *testing.T) {
	proxy := setupProxyServer()
	defer proxy.Close()

	server := setupHttpMockServer(nil)
	server_url := server.URL
	//Close server intentionally.
	server.Close()

	client := BuildHttpClientWithProxy()
	req, err := http.NewRequest("GET", server_url, nil)
	assert.NoError(t, err)
	assert.NotNil(t, req)

	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 400)
	assert.NotEmpty(t, resp)

	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.NoError(t, err)
	assert.NotEmpty(t, body)
}

func Test_HttpGetDefaultRequest(t *testing.T) {
	proxy := setupProxyServer()
	defer proxy.Close()

	server := setupHttpMockServer(nil)
	defer server.Close()

	resp, err := GetRequestTroughProxy(server.URL, "/")
	assert.Equal(t, resp.StatusCode, 200)
	assert.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.NoError(t, err)
	assert.Equal(t, string(body), "hello world")
}

func Test_HttpGetRequestCustomResponse(t *testing.T) {
	proxy := setupProxyServer()
	defer proxy.Close()

	const SERVER_RESPONSE = `"response": "AwesomeResponse"`
	server := setupHttpMockServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Add("Proxy_header", "golang_proxy")
		w.Write([]byte(SERVER_RESPONSE))
	}))
	defer server.Close()

	resp, err := GetRequestTroughProxy(server.URL, "/")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.NoError(t, err)
	assert.Equal(t, SERVER_RESPONSE, string(body))

	value, exists := resp.Header["Proxy_header"]
	assert.True(t, exists)
	assert.Equal(t, len(value), 1)
	assert.Equal(t, value[0], "golang_proxy")
}

func Test_HttpGetRequestCustomHeader(t *testing.T) {
	//Test if the client header is correctly forwarded to dest server by proxy

	proxy := setupProxyServer()
	defer proxy.Close()

	server := setupHttpMockServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		//Copy the received headers, and send it back
		for key, values := range req.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	request, _ := http.NewRequest("GET", server.URL+"/", nil)
	request.Header.Set("Customheader1", "header1")
	request.Header.Set("Customheader2", "header2")

	client := BuildHttpClientWithProxy()
	resp, err := client.Do(request)
	assert.NoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.Equal(t, string(body), "ok")

	assertHeaders := func(expected_key string, expected_value string, header http.Header) {
		value, exists := header[expected_key]
		assert.True(t, exists)
		assert.Contains(t, value, expected_value)
	}

	assertHeaders("Customheader1", "header1", resp.Header)
	assertHeaders("Customheader2", "header2", resp.Header)
}

func Test_ConnectMethodToProxy(t *testing.T) {
	proxy := setupProxyServer()
	defer proxy.Close()

	server := setupHttpMockServer(nil)
	defer server.Close()

	req, err := http.NewRequest("CONNECT", server.URL, nil)
	assert.NoError(t, err)

	//Create plain http.Client since the CONNECT method is for the proxy, not for the endpoint server
	client := http.Client{}
	resp, err := client.Do(req)
	assert.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.NoError(t, err)

	assert.Equal(t, string(body), string("hello world"))
	assert.Equal(t, resp.StatusCode, 200)
}
