package jsonapisdk

import (
	"fmt"
	"github.com/kucjac/jsonapi"
	"golang.org/x/text/language"
	display "golang.org/x/text/language/display"
	"net/http"
	"strings"
)

const (
	headerAcceptLanguage  = "Accept-Language"
	headerContentLanguage = "Content-Language"
	annotationSeperator   = ","
)

func (h *JSONAPIHandler) GetLanguage(
	req *http.Request,
	rw http.ResponseWriter,
) (tag language.Tag, ok bool) {
	tags, _, err := language.ParseAcceptLanguage(req.Header.Get(headerAcceptLanguage))
	if err != nil {
		errObj := jsonapi.ErrInvalidHeaderValue.Copy()
		errObj.Detail = err.Error()
		h.MarshalErrors(rw, errObj)
		return
	}
	tag, _, _ = h.LanguageMatcher.Match(tags...)
	rw.Header().Add(headerContentLanguage, tag.String())
	return tag, true
}

func (h *JSONAPIHandler) DisplaySupportedLanguages() []string {
	namer := display.Tags(language.English)
	var names []string = make([]string, len(h.SupportedLanguages))
	for i, lang := range h.SupportedLanguages {
		names[i] = namer.Name(lang)
	}
	return names
}

// CheckLanguage checks the language value within given scope's Value.
// It parses the field's value into language.Tag, and if no matching language is provided or the // language is not supported, it writes error to the response.
func (h *JSONAPIHandler) CheckValueLanguage(
	scope *jsonapi.Scope,
	rw http.ResponseWriter,
) (langtag language.Tag, ok bool) {
	lang, err := scope.GetLangtagValue()
	if err != nil {
		h.log.Errorf("Error while getting langtag from scope: '%v', Error: %v", scope.Struct.GetType(), err)
		h.MarshalInternalError(rw)
		return
	}

	if lang == "" {
		errObj := jsonapi.ErrInvalidInput.Copy()
		errObj.Detail = fmt.Sprintf("Provided object with no language code required. Supported languages: %v.", strings.Join(h.DisplaySupportedLanguages(), ","))
		h.MarshalErrors(rw, errObj)
		return
	} else {
		// Parse the language value from taken from the scope's value
		parseTag, err := language.Parse(lang)
		if err != nil {
			// if the error is a value error, then the subtag is invalid
			// this way the langtag should be a well-formed base tag
			if _, isValErr := err.(language.ValueError); !isValErr {
				// If provided language tag is not valid return a user error
				errObj := jsonapi.ErrInvalidInput.Copy()
				errObj.Detail = fmt.
					Sprintf("Provided invalid language tag: '%v'. Error: %v", lang, err)
				h.MarshalErrors(rw, errObj)
				return
			}
		}
		var confidence language.Confidence
		langtag, _, confidence = h.LanguageMatcher.Match(parseTag)
		// If the confidence is low or none send unsupported.
		if confidence <= language.Low {
			// language not supported
			errObj := jsonapi.ErrLanguageNotAcceptable.Copy()
			errObj.Detail = fmt.Sprintf("The language: '%s' is not supported. This document supports following languages: %v",
				lang,
				strings.Join(h.DisplaySupportedLanguages(), ","))
			h.MarshalErrors(rw, errObj)
			return
		}
	}

	err = scope.SetLangtagValue(langtag.String())
	if err != nil {
		h.log.Error(err)
		h.MarshalInternalError(rw)
		return
	}
	ok = true
	return
}

// HeaderContentLanguage sets the response Header 'Content-Language' to the lang tag provided in
// argument.
func (h *JSONAPIHandler) HeaderContentLanguage(rw http.ResponseWriter, langtag language.Tag) {
	rw.Header().Add(headerAcceptLanguage, langtag.String())
}
