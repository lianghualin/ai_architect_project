# LLM Tool Calling: A Complete Guide

This document explains how Large Language Models (LLMs) understand, decide, and use tools through function calling.

---

## Table of Contents

1. [How Does LLM Know About Tools?](#1-how-does-llm-know-about-tools)
2. [Why Does JSON Schema Work?](#2-why-does-json-schema-work)
3. [How Does LLM Decide When to Use Tools?](#3-how-does-llm-decide-when-to-use-tools)
4. [How Does LLM Know What Tools Are Available?](#4-how-does-llm-know-what-tools-are-available)
5. [Summary](#5-summary)

---

## 1. How Does LLM Know About Tools?

LLMs learn about tools through **function schemas** defined in JSON format. The schema acts as a contract that tells the LLM:

- What functions exist (name)
- What they do (description)
- What parameters they accept (parameters schema)
- Which parameters are required

### Example: Defining a Search Tool

```json
{
  "type": "function",
  "function": {
    "name": "search",
    "description": "Search the web for current information",
    "parameters": {
      "type": "object",
      "properties": {
        "keywords": {
          "type": "array",
          "items": {"type": "string"},
          "description": "Search terms to look up"
        },
        "max_results": {
          "type": "integer",
          "description": "Maximum number of results"
        }
      },
      "required": ["keywords"]
    }
  }
}
```

### The Tool Calling Flow

```
Step 1: Define tools in request
        ↓
Step 2: LLM responds with tool call
        ↓
Step 3: You execute tool & send result back
        ↓
Step 4: LLM generates final response
```

### Example Flow

**Step 1 - Request with Tools:**
```json
{
  "model": "gpt-5",
  "messages": [
    {"role": "user", "content": "What's the latest news about AI?"}
  ],
  "tools": [/* search tool schema */]
}
```

**Step 2 - LLM Returns Tool Call:**
```json
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": null,
      "tool_calls": [{
        "id": "call_abc123",
        "type": "function",
        "function": {
          "name": "search",
          "arguments": "{\"keywords\": [\"latest AI news 2026\"]}"
        }
      }]
    },
    "finish_reason": "tool_calls"
  }]
}
```

**Step 3 - Send Tool Result Back:**
```json
{
  "messages": [
    {"role": "user", "content": "What's the latest news about AI?"},
    {"role": "assistant", "content": null, "tool_calls": [...]},
    {
      "role": "tool",
      "tool_call_id": "call_abc123",
      "content": "{\"results\": [...]}"
    }
  ]
}
```

**Step 4:** LLM uses the tool result to formulate the final answer.

---

## 2. Why Does JSON Schema Work?

### LLMs Are Trained on Structured Data

LLMs have seen **billions of examples** of JSON during training:
- API documentation
- OpenAPI/Swagger specs
- Code repositories
- Stack Overflow answers

### The Schema Acts as a Contract

| Schema Part | What LLM Understands |
|-------------|---------------------|
| `name` | "I should output this exact string when calling" |
| `description` | "This is WHEN I should use this tool" |
| `parameters.properties` | "These are the inputs I need to provide" |
| `description` (per param) | "This is what each parameter means" |
| `required` | "I MUST include these, others are optional" |

### Fine-Tuning for Function Calling

Modern LLMs are specifically fine-tuned on function-calling examples:

```
Training Example 1:
User: "What's the weather in Tokyo?"
Assistant: {"name": "get_weather", "arguments": {"city": "Tokyo"}}

Training Example 2:
User: "Search for recent AI papers"
Assistant: {"name": "search", "arguments": {"keywords": ["AI papers 2024"]}}
```

### Key Insight

The LLM doesn't "execute" the function - it **generates structured text** that matches your schema. Your code then:

1. Parses the JSON output
2. Validates against schema
3. Calls the actual function
4. Returns result to LLM

```
LLM ──generates──> JSON ──parsed by──> Your Code ──calls──> Real Function
                                                                  │
LLM <──continues── Result <──────────────────────────────────────┘
```

---

## 3. How Does LLM Decide When to Use Tools?

### The Key: Knowledge Cutoff & Information Type

```
"Capital of France?"
 └─> Static fact, learned during training
 └─> LLM KNOWS this: "Paris"
 └─> ✗ No tool needed

"Weather in Paris?"
 └─> Real-time data, changes every hour
 └─> Training data is OLD (has cutoff date)
 └─> LLM DOESN'T KNOW current weather
 └─> ✓ Need search tool
```

### Decision Matrix

| Question Type | Characteristics | Tool Needed? |
|--------------|-----------------|--------------|
| Historical facts | Happened before training cutoff | ❌ No |
| Scientific facts | Stable knowledge | ❌ No |
| Math calculations | Can compute | ❌ No |
| **Current weather** | Changes hourly | ✅ Yes |
| **Today's news** | After training cutoff | ✅ Yes |
| **Stock prices** | Changes by second | ✅ Yes |
| **Live events** | Happening now | ✅ Yes |

### Key Signals LLM Looks For

**Words that trigger tool use:**
- "now", "today", "current", "latest", "right now"
- "weather", "stock price", "news"
- "live", "real-time", "happening"
- Any date after training cutoff

**Words that suggest no tool needed:**
- "what is", "explain", "define" (for concepts)
- Historical dates (before training cutoff)
- Math, logic, reasoning tasks
- General knowledge questions

### The Tool Description is Critical

```json
// ❌ Vague - LLM might misuse
{
  "name": "search",
  "description": "Search for information"
}

// ✅ Clear - LLM knows exactly when to use
{
  "name": "search",
  "description": "Search the web for information that requires real-time or current data: weather forecasts, current news, stock prices, sports scores, events happening today, or any facts that may have changed after January 2025. Do NOT use for general knowledge, historical facts, or scientific concepts."
}
```

### Decision Flow

```
                 User Question
                      │
                      ▼
        ┌─────────────────────────┐
        │   Is this real-time /   │
        │   current information?  │
        └─────────────────────────┘
                      │
           ┌──────────┴──────────┐
           │                     │
           ▼                     ▼
    ┌─────────────┐       ┌─────────────┐
    │     YES     │       │     NO      │
    │  (weather,  │       │  (capital,  │
    │   news,     │       │   history,  │
    │   stocks)   │       │   science)  │
    └─────────────┘       └─────────────┘
           │                     │
           ▼                     ▼
      Use Tool            Answer Directly
```

### Summary

> **LLM naturally reasons about what it knows vs. doesn't know; fine-tuning trains it to output the correct JSON format when it decides a tool is needed.**

---

## 4. How Does LLM Know What Tools Are Available?

### Answer: You Tell It Per Request

Tools are passed **in the context** at request time. The LLM doesn't "remember" tools - you define what's available each time.

### Method 1: `tools` Parameter (Modern Standard)

```json
{
  "model": "gpt-5",
  "messages": [...],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "search",
        "description": "...",
        "parameters": {...}
      }
    },
    {
      "type": "function",
      "function": {
        "name": "copy_file",
        "description": "...",
        "parameters": {...}
      }
    }
  ]
}
```

### Method 2: System Prompt (Old School / Manual)

```json
{
  "messages": [
    {
      "role": "system",
      "content": "You have access to the following tools:\n\n1. search(keywords): Search the web\n2. copy_file(src, dst): Copy a file\n\nWhen you need to use a tool, respond with JSON: {\"name\": \"...\", \"arguments\": {...}}"
    },
    {
      "role": "user",
      "content": "What's the weather in Tokyo?"
    }
  ]
}
```

### Tools Are Per-Request (No Persistent Memory)

```
Request 1: tools = [search, calculator]
  └─> LLM can use: search, calculator

Request 2: tools = [search, copy_file, delete_file]
  └─> LLM can use: search, copy_file, delete_file

Request 3: tools = []  (empty)
  └─> LLM can use: nothing, must answer directly
```

### Visual Summary

```
Your Application
      │
      │  POST /chat/completions
      │  {
      │    "messages": [...],
      │    "tools": [A, B, C]    ←── You define available tools
      │  }
      │
      ▼
┌─────────────────┐
│   LLM Service   │
│                 │
│  Context:       │
│  ┌───────────┐  │
│  │ System    │  │
│  │ + Tools   │  │  ←── Tools injected into context
│  │ + User    │  │
│  │   Message │  │
│  └───────────┘  │
│                 │
└─────────────────┘
      │
      ▼
   Response:
   "Use tool A" or "Direct answer"
```

---

## 5. Summary

### The Complete Picture

```
Tool-Calling Ability = Base Intelligence + Tool Schema + Fine-tuning
                              │                │             │
                              │                │             └─> Learn from examples
                              │                │                 when to trigger
                              │                │
                              │                └─> Here's what tools exist
                              │                    and what they do
                              │
                              └─> I can reason and generate
                                  structured output
```

### Key Takeaways

| Question | Answer |
|----------|--------|
| How does LLM know about tools? | Through JSON function schemas passed in the request |
| Why does JSON schema work? | LLMs are trained on JSON + fine-tuned on function-calling examples |
| How does LLM decide when to use tools? | Recognizes patterns (real-time vs static) + reads tool descriptions |
| How does LLM know what tools are available? | You tell it per request via `tools` parameter or system prompt |

### One-Line Summary

> **You define tools in JSON schema, pass them with each request, and the LLM—trained on thousands of examples—decides when to call them based on whether it needs external/real-time information.**
