package soap

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

// Params type is used to set the params in soap request
type Params map[string]string

// NewClient return new *Client to handle the requests with the WSDL
func NewClient(wsdl string) (*Client, error) {
	_, err := url.Parse(wsdl)
	if err != nil {
		return nil, err
	}

	d, err := getWsdlDefinitions(wsdl)
	if err != nil {
		return nil, err
	}

	c := &Client{
		WSDL:        wsdl,
		URL:         strings.TrimSuffix(d.TargetNamespace, "/"),
		Definitions: d,
	}

	return c, nil
}

// Client struct hold all the informations about WSDL,
// request and response of the server
type Client struct {
	WSDL         string
	URL          string
	Method       string
	Params       Params
	HeaderName   string
	HeaderParams Params
	Definitions  *wsdlDefinitions
	Body         []byte
	Header       []byte

	payload []byte
}

func (c *Client) GetLastRequest() []byte {
	return c.payload
}

// Call call's the method m with Params p
func (c *Client) Call(m string, p Params) (err error) {
	c.Method = m
	c.Params = p

	c.payload, err = xml.MarshalIndent(c, "", "")
	if err != nil {
		return fmt.Errorf("MarshalIndent failed: %s", err.Error())
	}

	b, err := c.doRequest(c.Definitions.Services[0].Ports[0].SoapAddresses[0].Location)
	if err != nil {
		return fmt.Errorf("doRequest failed: %s", err.Error())
	}

	var soap SoapEnvelope
	err = xml.Unmarshal(b, &soap)

	c.Body = soap.Body.Contents
	c.Header = soap.Header.Contents

	if err != nil {
		return fmt.Errorf("Unmarshal response failed: %s", err.Error())
	}
	return nil
}

// Unmarshal get the body and unmarshal into the interface
func (c *Client) Unmarshal(v interface{}) error {
	if len(c.Body) == 0 {
		return fmt.Errorf("Body is empty")
	}

	var f Fault
	xml.Unmarshal(c.Body, &f)
	if f.Code != "" {
		return fmt.Errorf("[%s]: %s", f.Code, f.Description)
	}

	return xml.Unmarshal(c.Body, v)
}

// doRequest makes new request to the server using the c.Method, c.URL and the body.
// body is enveloped in Call method
func (c *Client) doRequest(url string) ([]byte, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(c.payload))
	if err != nil {
		return nil, err
	}
	// TODO: Refactor later for accepting self signed certificate
	// client := &http.Client{}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	req.ContentLength = int64(len(c.payload))

	req.Header.Add("Content-Type", "text/xml;charset=UTF-8")
	req.Header.Add("Accept", "text/xml")
	req.Header.Add("SOAPAction", fmt.Sprintf("%s/%s", c.URL, c.Method))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// TODO: Handle HTTP error first

	return ioutil.ReadAll(resp.Body)
}

// SoapEnvelope struct
type SoapEnvelope struct {
	XMLName struct{} `xml:"Envelope"`
	Header  SoapHeader
	Body    SoapBody
}

// SoapHeader struct
type SoapHeader struct {
	XMLName  struct{} `xml:"Header"`
	Contents []byte   `xml:",innerxml"`
}

// SoapBody struct
type SoapBody struct {
	XMLName  struct{} `xml:"Body"`
	Contents []byte   `xml:",innerxml"`
}
