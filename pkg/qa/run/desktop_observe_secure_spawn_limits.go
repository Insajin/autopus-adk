package run

import "github.com/insajin/autopus-adk/pkg/qa/desktopobserve"

func desktopObservationEnvelopeLimit() int {
	return desktopobserve.MaxEnvelopeBytes
}

func desktopobserveEnvelopeTooLarge() error {
	return desktopobserve.ErrEnvelopeTooLarge
}
