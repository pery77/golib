//go:build js

package device

import "errors"

// Language models don't run in a browser: GoLib's native library for them,
// golibllm, is for the desktop.

var errNoLanguageModels = errors.New("language models run on the desktop only, not in a browser")

func LoadLanguageModel(path string, s LanguageModelSettings) (LanguageModel, error) {
	usesLanguageModel = LanguageModelMarker
	return LanguageModel{}, errNoLanguageModels
}

func FreeLanguageModel(LanguageModel) {}

func LanguageModelPrompt(LanguageModel, []string, []string) (string, error) {
	return "", errNoLanguageModels
}

func StartLanguageModelReply(LanguageModel, string, Sampling) error { return errNoLanguageModels }

func NextLanguageModelPiece(LanguageModel) (string, bool, error) {
	return "", true, errNoLanguageModels
}
