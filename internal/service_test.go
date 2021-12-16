package internal_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/reddec/ingress-dashboard/internal"
	"github.com/stretchr/testify/require"
)

func TestLoadDefinitions(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	err = ioutil.WriteFile(filepath.Join(tempDir, "demo.yaml"), []byte(`
---
name: Some site
namespace: external links # used in 'from'
urls:
  - https://example.com
---
name: Google
namespace: external links
logo_url: https://www.google.ru/favicon.ico
description: |
  Well-known search engine
urls:
  - https://google.com
  - http://example.com
`), 0755)
	require.NoError(t, err)

	list, err := internal.LoadDefinitions(tempDir)
	require.NoError(t, err)
	require.Len(t, list, 2)

	require.Equal(t, internal.Ingress{
		Name:      "Some site",
		Namespace: "external links",
		Refs: []internal.Ref{
			{URL: "https://example.com", Static: true},
		},
	}, list[0])

	require.Equal(t, internal.Ingress{
		Name:        "Google",
		Namespace:   "external links",
		Description: "Well-known search engine\n",
		LogoURL:     "https://www.google.ru/favicon.ico",
		Refs: []internal.Ref{
			{URL: "https://google.com", Static: true},
			{URL: "http://example.com", Static: true},
		},
	}, list[1])
}
