package xai

import "encoding/json"

// CreateConversationResponse is the response from creating a conversation
type CreateConversationResponse struct {
	Result CreateConversationResponseResult `json:"result"`
}

// CreateConversationResponseResult contains either a Conversation or Response data
type CreateConversationResponseResult struct {
	Conversation Conversation `json:"conversation,omitempty"`
	Response     Response     `json:"response,omitempty"`
}

// ContinueConversationResponse is the response from continuing a conversation
type ContinueConversationResponse struct {
	Result ContinueConversationResponseResult `json:"result"`
}

// ContinueConversationResponseResult contains response data for ongoing conversations
type ContinueConversationResponseResult struct {
	Response         Response         `json:"response,omitempty"`
	WebSearchResults WebSearchResults `json:"webSearchResults,omitempty"`
	XSearchResults   XSearchResults   `json:"xSearchResults,omitempty"`
	Token            string           `json:"token,omitempty"`
	IsThinking       bool             `json:"isThinking,omitempty"`
	IsSoftStop       bool             `json:"isSoftStop,omitempty"`
	ResponseID       string           `json:"responseId,omitempty"`
	MessageTag       string           `json:"messageTag,omitempty"`
	MessageStepID    int              `json:"messageStepId,omitempty"`
}

// Conversation holds conversation metadata
type Conversation struct {
	ConversationID   string   `json:"conversationId"`
	Title            string   `json:"title"`
	Starred          bool     `json:"starred"`
	CreateTime       string   `json:"createTime"`
	ModifyTime       string   `json:"modifyTime"`
	SystemPromptName string   `json:"systemPromptName"`
	Temporary        bool     `json:"temporary"`
	MediaTypes       []string `json:"mediaTypes,omitempty"`
}

// Response contains various nested response types
type Response struct {
	ResponseID string `json:"responseId,omitempty"`
	Token      string `json:"token,omitempty"`
	IsThinking bool   `json:"isThinking,omitempty"`
	IsSoftStop bool   `json:"isSoftStop,omitempty"`

	UserResponse  UserResponse  `json:"userResponse,omitempty"`
	ModelResponse ModelResponse `json:"modelResponse,omitempty"`
	FinalMetadata FinalMetadata `json:"finalMetadata,omitempty"`

	StreamingImageGenerationResponse StreamingImageGenerationResponse `json:"streamingImageGenerationResponse,omitempty"`
}

// WebSearchResults contains web search result data
type WebSearchResults struct {
	Results []SearchResult `json:"results,omitempty"`
}

// SearchResult represents an individual web search result
type SearchResult struct {
	URL              string `json:"url,omitempty"`
	Title            string `json:"title,omitempty"`
	Preview          string `json:"preview,omitempty"`
	SearchEngineText string `json:"searchEngineText,omitempty"`
	Description      string `json:"description,omitempty"`
	SiteName         string `json:"siteName,omitempty"`
	MetadataTitle    string `json:"metadataTitle,omitempty"`
	Creator          string `json:"creator,omitempty"`
	Image            string `json:"image,omitempty"`
	Favicon          string `json:"favicon,omitempty"`
	CitationID       string `json:"citationId,omitempty"`
}

// XSearchResults contains X (Twitter-like) search result data
type XSearchResults struct {
	Results []XSearchResult `json:"results,omitempty"`
}

// XSearchResult represents an individual X search result
type XSearchResult struct {
	Username        string      `json:"username,omitempty"`
	Name            string      `json:"name,omitempty"`
	Text            string      `json:"text,omitempty"`
	CreateTime      string      `json:"createTime,omitempty"`
	ProfileImageURL string      `json:"profileImageUrl,omitempty"`
	PostID          string      `json:"postId,omitempty"`
	CitationID      string      `json:"citationId,omitempty"`
	Parent          interface{} `json:"parent,omitempty"`
	Quote           interface{} `json:"quote,omitempty"`
	ViewCount       int         `json:"viewCount,omitempty"`
}

// UserResponse represents a human user's response
type UserResponse struct {
	ResponseID              string   `json:"responseId,omitempty"`
	Message                 string   `json:"message,omitempty"`
	Sender                  string   `json:"sender,omitempty"`
	CreateTime              string   `json:"createTime,omitempty"`
	Manual                  bool     `json:"manual,omitempty"`
	Partial                 bool     `json:"partial,omitempty"`
	Shared                  bool     `json:"shared,omitempty"`
	Query                   string   `json:"query,omitempty"`
	QueryType               string   `json:"queryType,omitempty"`
	WebSearchResults        []string `json:"webSearchResults,omitempty"`
	XpostIds                []string `json:"xpostIds,omitempty"`
	Xposts                  []string `json:"xposts,omitempty"`
	GeneratedImageUrls      []string `json:"generatedImageUrls,omitempty"`
	ImageAttachments        []string `json:"imageAttachments,omitempty"`
	FileAttachments         []string `json:"fileAttachments,omitempty"`
	CardAttachmentsJson     []string `json:"cardAttachmentsJson,omitempty"`
	FileUris                []string `json:"fileUris,omitempty"`
	FileAttachmentsMetadata []string `json:"fileAttachmentsMetadata,omitempty"`
	IsControl               bool     `json:"isControl,omitempty"`
	Steps                   []string `json:"steps,omitempty"`
	MediaTypes              []string `json:"mediaTypes,omitempty"`
	WebpageUrls             []string `json:"webpageUrls,omitempty"`
}

// ModelResponse represents an AI model's response
type ModelResponse struct {
	ResponseID              string   `json:"responseId,omitempty"`
	Message                 string   `json:"message,omitempty"`
	Sender                  string   `json:"sender,omitempty"`
	CreateTime              string   `json:"createTime,omitempty"`
	ParentResponseID        string   `json:"parentResponseId,omitempty"`
	Manual                  bool     `json:"manual,omitempty"`
	Partial                 bool     `json:"partial,omitempty"`
	Shared                  bool     `json:"shared,omitempty"`
	Query                   string   `json:"query,omitempty"`
	QueryType               string   `json:"queryType,omitempty"`
	WebSearchResults        []string `json:"webSearchResults,omitempty"`
	XpostIds                []string `json:"xpostIds,omitempty"`
	Xposts                  []string `json:"xposts,omitempty"`
	GeneratedImageUrls      []string `json:"generatedImageUrls,omitempty"`
	ImageAttachments        []string `json:"imageAttachments,omitempty"`
	FileAttachments         []string `json:"fileAttachments,omitempty"`
	CardAttachmentsJson     []string `json:"cardAttachmentsJson,omitempty"`
	FileUris                []string `json:"fileUris,omitempty"`
	FileAttachmentsMetadata []string `json:"fileAttachmentsMetadata,omitempty"`
	IsControl               bool     `json:"isControl,omitempty"`
	Steps                   []string `json:"steps,omitempty"`
	MediaTypes              []string `json:"mediaTypes,omitempty"`
	WebpageUrls             []string `json:"webpageUrls,omitempty"`
}

// FinalMetadata contains response completion metadata
type FinalMetadata struct {
	FollowUpSuggestions []FollowUpSuggestion `json:"followUpSuggestions,omitempty"`
	FeedbackLabels      []string             `json:"feedbackLabels,omitempty"`
	ToolsUsed           ToolsUsed            `json:"toolsUsed"`
	Disclaimer          string               `json:"disclaimer,omitempty"`
}

// FollowUpSuggestion represents suggested follow-up actions
type FollowUpSuggestion struct {
	Properties    SuggestionProperties `json:"properties"`
	Label         string               `json:"label,omitempty"`
	ToolOverrides ToolOverrides        `json:"toolOverrides"`
}

// SuggestionProperties contains follow-up suggestion details
type SuggestionProperties struct {
	MessageType  string `json:"messageType,omitempty"`
	FollowUpType string `json:"followUpType,omitempty"`
}

// ToolOverrides represents tool override settings
type ToolOverrides struct {
}

// ToolsUsed represents tools utilized in the response
type ToolsUsed struct {
}

// StreamingImageGenerationResponse represents an image generation progress update
type StreamingImageGenerationResponse struct {
	ImageID  string `json:"imageId,omitempty"`
	ImageURL string `json:"imageUrl,omitempty"`
	Seq      int    `json:"seq,omitempty"`
	Progress int    `json:"progress,omitempty"`
}

// JSON formatters remain unchanged...
func (r CreateConversationResponse) String() string {
	j, _ := json.Marshal(r)
	return string(j)
}

func (r CreateConversationResponseResult) String() string {
	j, _ := json.Marshal(r)
	return string(j)
}

func (r ContinueConversationResponse) String() string {
	j, _ := json.Marshal(r)
	return string(j)
}

func (r ContinueConversationResponseResult) String() string {
	j, _ := json.Marshal(r)
	return string(j)
}

func (c Conversation) String() string {
	j, _ := json.Marshal(c)
	return string(j)
}

func (rd Response) String() string {
	j, _ := json.Marshal(rd)
	return string(j)
}

func (ur UserResponse) String() string {
	j, _ := json.Marshal(ur)
	return string(j)
}

func (mr ModelResponse) String() string {
	j, _ := json.Marshal(mr)
	return string(j)
}

func (fm FinalMetadata) String() string {
	j, _ := json.Marshal(fm)
	return string(j)
}

func (fus FollowUpSuggestion) String() string {
	j, _ := json.Marshal(fus)
	return string(j)
}

func (sp SuggestionProperties) String() string {
	j, _ := json.Marshal(sp)
	return string(j)
}

func (to ToolOverrides) String() string {
	j, _ := json.Marshal(to)
	return string(j)
}

func (tu ToolsUsed) String() string {
	j, _ := json.Marshal(tu)
	return string(j)
}
