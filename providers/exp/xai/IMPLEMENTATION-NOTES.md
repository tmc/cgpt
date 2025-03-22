# Grok-3 API Implementation Notes

## Overview

This implementation provides a pure Go client for the Grok-3 API. It allows CGPT to communicate directly with the Grok API service without relying on external shell scripts or tools.

## Key Features

1. **Authentication**: Uses cookie-based authentication with the Grok website
2. **Conversation Management**: Handles creating new conversations and managing multi-turn conversations
3. **Streaming Support**: Implements token-by-token streaming for improved user experience
4. **Error Handling**: Robust error handling for network, authentication, and API issues
5. **Unit Tests**: Comprehensive unit tests for core functionality

## Implementation Details

### Authentication

Authentication is handled using cookies extracted from a logged-in session on the Grok website. At minimum, the `sso` cookie is required, which is set via the `XAI_SESSION_COOKIE` environment variable. Additional cookies can be provided for enhanced authentication.

### HTTP Client Configuration

The implementation uses a standard Go `http.Client` with HTTP/2 support, which is required for proper API communication. The client can be customized when creating a new Grok-3 instance.

### API Workflow

1. **Initialization**: Create a new Grok-3 instance with required cookies and options
2. **Conversation Creation**: When a user sends their first message, a new conversation is created
3. **Response Generation**: The API is called to generate responses, with support for both first and follow-up messages
4. **Streaming Processing**: For streaming responses, tokens are parsed and sent to the callback function
5. **Non-Streaming Processing**: For non-streaming responses, the full response is accumulated and returned

### Conversation Management

The implementation maintains a conversation ID for each session, allowing for stateful interactions with the Grok API. Follow-up messages are sent to the same conversation to maintain context.

## Multi-Turn Conversations

The implementation supports multi-turn conversations with the Grok API. This allows users to maintain context across multiple interactions.

### Multi-Turn Implementation Details

1. **First Message**: The first message creates a new conversation and obtains a conversation ID
2. **Follow-up Messages**: Subsequent messages are sent to the same conversation ID
3. **Message Requirements**: The Grok API requires non-empty message content for all messages in a conversation
4. **Context Preservation**: The API maintains context between messages in the same conversation

### Multi-Turn API Flow

1. **Start Conversation**: Call `startNewConversation()` with the first message
2. **Initial Response**: Get the initial response with `getConversationResponse()` (must include message content)
3. **Follow-up Messages**: For subsequent messages, call `getConversationResponse()` with the new message content
4. **Streaming Support**: Both initial and follow-up messages support streaming responses

## Potential Issues and Solutions

1. **Cookie Expiration**: Cookies will expire after a period of time. Users need to refresh cookies periodically.
2. **IP Address Binding**: Cookies may be bound to specific IP addresses. Ensure the same IP is used for extraction and API calls.
3. **Cloudflare Protection**: The Grok website uses Cloudflare protection. The `cf_clearance` cookie may be required in some cases.
4. **Rate Limiting**: The API may impose rate limits. Implement backoff/retry logic if needed.
5. **Response Parsing**: The streaming response format may change. Keep the parsing logic updated.
6. **Empty Messages**: The API returns 400 Bad Request for empty messages. Always include message content for all API calls.
7. **Timeout Issues**: API responses can sometimes be slow. Implement appropriate timeouts and handle errors gracefully.

## HTTP/2 Requirements

The Grok API requires HTTP/2 for proper communication. The implementation enforces this by default and will fail with an error if HTTP/2 cannot be negotiated. HTTP/2 is especially important for streaming responses.

## Future Improvements

1. **Conversation History**: Add support for fetching conversation history
2. **Error Recovery**: Implement automatic retry with backoff for transient errors
3. **Better Cookie Management**: Develop a more user-friendly way to manage authentication cookies
4. **Proxy Support**: Add support for proxies to handle IP address restrictions
5. **Response Metadata**: Parse and expose additional response metadata from the API
6. **Performance Optimization**: Optimize for faster response times and better resource utilization