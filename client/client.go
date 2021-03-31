package client

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
)

const (
	defaultBaseURL       = "http://drbd.io:3030/api/v1/best"
	defaultOsReleasePath = "/etc/os-release"
)

type Option func(*Client) error

type Client struct {
	httpClient    *http.Client
	baseURL       *url.URL
	osReleasePath string
}

func WithBaseURL(baseURL *url.URL) Option {
	return func(c *Client) error {
		c.baseURL = baseURL
		return nil
	}
}

func WithOSReleasePath(osReleasePath string) Option {
	return func(c *Client) error {
		c.osReleasePath = osReleasePath
		return nil
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) error {
		c.httpClient = httpClient
		return nil
	}
}

func NewClient(options ...Option) *Client {
	parsedBaseURL, err := url.Parse(defaultBaseURL)
	if err != nil {
		// since this is the hard coded default, panic when it is invalid
		panic(fmt.Sprintf("default base URL not valid: %v", err))
	}
	cli := &Client{
		baseURL:       parsedBaseURL,
		osReleasePath: defaultOsReleasePath,
		httpClient:    &http.Client{},
	}
	for _, o := range options {
		o(cli)
	}

	return cli
}

// BestKmod queries the API for the best package to install for a given kernel.
// It takes the desired kernel version (output of `uname -r` or something
// compatible) and returns the package file name of the best fitting kmod.
func (c *Client) BestKmod(unameR string) (string, error) {
	bestKmodUrl := c.baseURL.String() + "/" + unameR
	osRelease, err := os.Open(c.osReleasePath)
	if err != nil {
		return "", fmt.Errorf("failed to open /etc/os-release: %w", err)
	}
	resp, err := c.httpClient.Post(bestKmodUrl, "text/plain", osRelease)
	if err != nil {
		return "", fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read from response body: %w", err)
	}

	return string(bytes), nil
}
