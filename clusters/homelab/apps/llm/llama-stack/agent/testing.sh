#!/bin/bash

curl http://localhost:8000/v1/completions   -H "Content-Type: application/json"   -H "Authorization: Bearer $OPENAI_API_KEY"   -d '{
    "model": "vllm",
    "prompt": "In which country is berlin located?",
    "max_tokens": 50,
    "temperature": 0
  }'

# {"id":"cmpl-1234","object":"text_completion","created":1747753159,"model":"vllm","choices":[{"text":"Berlin is the capital city of Germany.","index":0,"logprobs":null,"finish_reason":"stop"}]}

curl http://localhost:8000/v1/completions   -H "Content-Type: application/json"   -H "Authorization: Bearer $OPENAI_API_KEY"   -d '{
    "model": "vllm",
    "prompt": "What is 40+30?",
    "max_tokens": 7,
    "temperature": 0
  }'

# {"id":"cmpl-1234","object":"text_completion","created":1747749081,"model":"vllm","choices":[{"text":"[{\"name\": \"calculator\", \"arguments\": {\"x\": 40, \"y\": 30, \"operation\": \"add\"}}]","index":0,"logprobs":null,"finish_reason":"stop"}]}
