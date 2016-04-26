package main

import (
	"golang.org/x/oauth2"
)

type (
	// This is a simple type satisfying the oauth2.TokenSource interface.
	tokenSource string
)

// Return an  *oauth2.Token.
func (t tokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: string(t),
	}

	return token, nil
}
