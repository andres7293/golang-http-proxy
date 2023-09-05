package proxy

import (
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type Proxy struct {
	server http.Server
}

func (p *Proxy) Start(addr string) error {
	p.server = http.Server{
		Addr:    addr,
		Handler: p,
	}
	return p.server.ListenAndServe()
}

func (p *Proxy) Close() error {
	return p.server.Close()
}

func (h *Proxy) ServeHTTP(w http.ResponseWriter, clientRequest *http.Request) {
	r := clientRequest
	log.Printf("Client connected at: %s %s\n", r.Method, r.RequestURI)
	if r.Method == "CONNECT" {
		Https_proxy(w, r)
	} else {
		Http_proxy(w, r)
	}
}

func SendErrorAtServingRequest(w http.ResponseWriter, err error) {
	w.WriteHeader(400)
	w.Write([]byte(err.Error()))
}

func Http_proxy(w http.ResponseWriter, clientRequest *http.Request) {
	r := clientRequest
	proxyRequest, err := http.NewRequest(r.Method, "http://"+r.Host, r.Body)
	//Copy client headers into proxyRequest headers
	for key, value := range r.Header {
		proxyRequest.Header[key] = value
	}
	//Make HTTP request in behalf of the client
	proxyClient := &http.Client{}
	proxyResponse, err := proxyClient.Do(proxyRequest)
	if err != nil {
		log.Printf("%s\n", err)
		SendErrorAtServingRequest(w, err)
		return
	}
	defer proxyResponse.Body.Close()

	//Copy the header received by the server
	for key, values := range proxyResponse.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(proxyResponse.StatusCode)
	io.Copy(w, proxyResponse.Body)
}

func getRemoteAddr(r *http.Request) string {
	if r.Host == "http:" {
		temp := strings.Replace(r.RequestURI, "http://", "", 1)
		return strings.Replace(temp, "/", "", 1)
	}
	return r.Host
}

func Https_proxy(w http.ResponseWriter, clientRequest *http.Request) {
	r := clientRequest
	proxyConn, err := net.DialTimeout("tcp", getRemoteAddr(r), 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		SendErrorAtServingRequest(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	go transfer(proxyConn, clientConn)
	go transfer(clientConn, proxyConn)
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}
