package client

import (
	"fmt"
	"io"
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

func NewClient(options ...Option) (*Client, error) {
	parsedBaseURL, err := url.Parse(defaultBaseURL)
	if err != nil {
		return nil, err
	}

	cli := &Client{
		baseURL:       parsedBaseURL,
		osReleasePath: defaultOsReleasePath,
		httpClient:    &http.Client{},
	}

	for _, o := range options {
		if err := o(cli); err != nil {
			return nil, err
		}
	}

	return cli, nil
}

func NewClientMust(options ...Option) *Client {
	cli, err := NewClient(options...)
	if err != nil {
		panic(err)
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

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API call failed (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read from response body: %w", err)
	}

	return string(bytes), nil
}
