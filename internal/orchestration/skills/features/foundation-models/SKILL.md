---
name: "foundation-models"
description: "On-device AI with Apple FoundationModels framework: text generation, streaming responses, structured output with @Generable. Use when adding on-device AI features, text generation, or ML-powered content creation. Triggers: FoundationModels, @Generable, LanguageModelSession, on-device AI, text generation."
---
# Foundation Models

APPLE ON-DEVICE AI (FoundationModels — iOS 26+):
FRAMEWORK: import FoundationModels

AVAILABILITY CHECK (MANDATORY — model may not be available on all devices):
guard case .available = SystemLanguageModel.default.availability else {
    // Show "This feature requires Apple Intelligence" message
    return
}

BASIC TEXT GENERATION:
let session = LanguageModelSession()
let response = try await session.respond(to: "Summarize this text: \(userText)")
print(response.content)

STREAMING GENERATION:
let stream = session.streamResponse(to: prompt)
for try await partial in stream {
    displayText += partial.text
}

STRUCTURED OUTPUT with @Generable:
@Generable
struct RecipeSuggestion {
    @Guide(description: "Name of the dish") var name: String
    @Guide(description: "Estimated prep time in minutes") var prepTime: Int
    @Guide(description: "Main ingredients") var ingredients: [String]
}

let session = LanguageModelSession()
let recipe: RecipeSuggestion = try await session.respond(
    to: "Suggest a quick pasta dish",
    generating: RecipeSuggestion.self
)

SESSION INSTRUCTIONS (system prompt):
let session = LanguageModelSession(instructions: "You are a helpful cooking assistant. Keep responses concise.")

GUARDRAILS:
- Model output is filtered by Apple's safety system
- No internet required — fully on-device
- Context window is limited (~4K tokens typical) — keep prompts concise
- Use session.respond() for single turns, keep session for multi-turn conversations
