package main

import (
	_ "embed"
)

//go:embed assets/socials.png
var defaultSocialsPNG []byte

// appSocials holds the decoded socials image shown in full-screen mode.
var appSocials *logoSource

// initAppSocials decodes the embedded socials.png once at startup.
func initAppSocials() error {
	src, err := decodeLogoImage(defaultSocialsPNG)
	if err != nil {
		return err
	}
	appSocials = src
	return nil
}
