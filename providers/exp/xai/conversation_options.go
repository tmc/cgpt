package xai

// ConversationOptions is a map of options that can be passed to the conversation API.
type ConversationOptions map[string]any

// ConversationResponseOption is a function that can be used to set options on a conversation request.
type ConversationResponseOption = func(ConversationOptions)

// TODO: distinguish more on what can be included initially vs subsequently.

// WithThinking sets the isReasoning flag to true.
func WithThinking(thinking bool) ConversationResponseOption {
	return func(req ConversationOptions) {
		req["isReasoning"] = true
	}
}

// WithDeepSearch sets the deepsearchPreset to "deepsearch".
func WithDeepSearch() ConversationResponseOption {
	return func(req ConversationOptions) {
		req["deepsearchPreset"] = "deepsearch"
	}
}

// WithDeeperSearch sets the deepsearchPreset to "deepersearch".
func WithDeeperSearch() ConversationResponseOption {
	return func(req ConversationOptions) {
		req["deepsearchPreset"] = "deepersearch"
	}
}

// WithTemporary sets the temporary flag to true.
func WithTemporary(temporary bool) ConversationResponseOption {
	return func(req ConversationOptions) {
		req["temporary"] = temporary
	}
}

// WithSystemPromptName sets the systemPromptName to the given string. Must be a valid system prompt name.
func WithSystemPromptName(systemPromptName string) ConversationResponseOption {
	return func(req ConversationOptions) {
		req["systemPromptName"] = systemPromptName
	}
}

// WithDisableSearch sets the disableSearch flag to the given boolean.
func WithDisableSearch(disable bool) ConversationResponseOption {
	return func(req ConversationOptions) {
		req["disableSearch"] = disable
	}
}

// WithCustomPersonality sets the customPersonality to the given string.
func WithCustomPersonality(personality string) ConversationResponseOption {
	return func(req ConversationOptions) {
		req["customPersonality"] = personality
	}
}

// Note: this may have no effect
func WithCustomInstructions(instructions string) ConversationResponseOption {
	return func(req ConversationOptions) {
		req["customInstructions"] = instructions
	}
}

// WithWebpageUrls sets the webpageUrls to the given list of strings.
func WithWebpageUrls(urls []string) ConversationResponseOption {
	return func(req ConversationOptions) {
		req["webpageUrls"] = urls
	}
}

// WithFileAttachments sets the fileAttachments to the given list of URLs.
func WithFileAttachments(files []string) ConversationResponseOption {
	return func(req ConversationOptions) {
		req["fileAttachments"] = files
	}
}

// WithImageAttachments sets the imageAttachments to the given list of URLs.
func WithImageAttachments(images []string) ConversationResponseOption {
	return func(req ConversationOptions) {
		req["imageAttachments"] = images
	}
}

// WithForceConcise sets the forceConcise flag to the given boolean.
func WithForceConcise(force bool) ConversationResponseOption {
	return func(req ConversationOptions) {
		req["forceConcise"] = force
	}
}
