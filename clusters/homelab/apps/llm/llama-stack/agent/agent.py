#!/usr/bin/env python3

import os
import fire
from termcolor import colored
from llama_stack_client import LlamaStackClient, Agent, AgentEventLogger

# Set up logging for the calculator tool
import logging
from llama_stack_client.lib.agents.client_tool import client_tool

logging.basicConfig(level=logging.WARNING)
logger = logging.getLogger(__name__)

def check_model_is_available(client: LlamaStackClient, model: str):
    available_models = [
        model.identifier
        for model in client.models.list()
        if model.model_type == "llm" and "guard" not in model.identifier
    ]

    if model not in available_models:
        print(
            colored(
                f"Model `{model}` not found. Available models:\n\n{available_models}\n",
                "red",
            )
        )
        return False

    return True


def get_any_available_model(client: LlamaStackClient):
    available_models = [
        model.identifier
        for model in client.models.list()
        if model.model_type == "llm" and "guard" not in model.identifier
    ]
    if not available_models:
        print(colored("No available models.", "red"))
        return None

    return available_models[0]

@client_tool
def calculator(x: float, y: float, operation: str) -> dict:
    """Simple calculator tool that performs basic math operations.

    :param x: First number to perform operation on
    :param y: Second number to perform operation on
    :param operation: Mathematical operation to perform ('add', 'subtract', 'multiply', 'divide')
    :returns: Dictionary containing success status and result or error message
    """
    logger.debug(f"Calculator called with: x={x}, y={y}, operation={operation}")
    try:
        if operation == "add":
            result = float(x) + float(y)
        elif operation == "subtract":
            result = float(x) - float(y)
        elif operation == "multiply":
            result = float(x) * float(y)
        elif operation == "divide":
            if float(y) == 0:
                return {"success": False, "error": "Cannot divide by zero"}
            result = float(x) / float(y)
        else:
            return {"success": False, "error": "Invalid operation"}

        logger.debug(f"Calculator result: {result}")
        return {"success": True, "result": result}
    except Exception as e:
        logger.error(f"Calculator error: {str(e)}")
        return {"success": False, "error": str(e)}

def main(host: str, port: int, model_id: str | None = None):
    client = LlamaStackClient(base_url=f"http://{host}:{port}")

    api_key = ""
    engine = "tavily"
    if "TAVILY_SEARCH_API_KEY" in os.environ:
        api_key = os.getenv("TAVILY_SEARCH_API_KEY")
    elif "BRAVE_SEARCH_API_KEY" in os.environ:
        api_key = os.getenv("BRAVE_SEARCH_API_KEY")
        engine = "brave"
    else:
        print(
            colored(
                "Warning: TAVILY_SEARCH_API_KEY or BRAVE_SEARCH_API_KEY is not set; Web search will not work",
                "yellow",
            )
        )

    if model_id is None:
        model_id = get_any_available_model(client)
        if model_id is None:
            return
    else:
        if not check_model_is_available(client, model_id):
            return

    agent = Agent(
        client,
        model=model_id,
        instructions="You are a helpful assistant. Use the tools you have access to for providing relevant answers.",
        sampling_params={
            "strategy": {"type": "top_p", "temperature": 1.0, "top_p": 0.9},
        },
        tools=[
            calculator,
        ],
    )
    session_id = agent.create_session("test-session")
    print(f"Created session_id={session_id} for Agent({agent.agent_id})")

    user_prompts = [
        "What is 40+30?",
        "What is 100 divided by 4?",
        "What is 50 multiplied by 2?"
    ]
    for prompt in user_prompts:
        print(colored(f"User> {prompt}", "cyan"))
        response = agent.create_turn(
            messages=[{"role": "user", "content": prompt}],
            session_id=session_id,
        )

        for log in AgentEventLogger().log(response):
            log.print()


if __name__ == "__main__":
    fire.Fire(main)
