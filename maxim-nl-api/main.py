# main.py
from fastapi import FastAPI, HTTPException
from fastapi.responses import PlainTextResponse
from pydantic import BaseModel, Field
from crewai import Agent, Task, Crew, LLM # Import LLM
from crewai_tools import NL2SQLTool
import uvicorn

# --- Pydantic Models ---
class NLQueryRequest(BaseModel):
    nl_query: str = Field(..., description="The natural language query to convert.")
    db_uri: str = Field(..., description="The database connection URI.")

class SQLQueryResponse(BaseModel):
    sql_query: str = Field(..., description="The generated SQL query.")

# --- Initialize the FastAPI App ---
app = FastAPI(
    title="Maxim NL-to-SQL API",
    description="Converts natural language queries to SQL using CrewAI NL2SQLTool",
    version="1.0.0"
)


gemini_llm = LLM(
    model="gemini/gemini-2.5-flash", 
    temperature=0.0
)

@app.post("/generate-sql", response_class=PlainTextResponse)
async def generate_sql(request: NLQueryRequest):
    """
    Accepts a natural language query and DB URI, returns the generated SQL.
    """
    try:
        # 1. Initialize the tool
        #
        #    *** THIS IS THE FIX ***
        #    Pass the LLM to the tool itself.
        #
        nl2sql = NL2SQLTool(db_uri=request.db_uri, llm=gemini_llm)

        # 2. Create an Agent that uses the same LLM
        sql_agent = Agent(
            role='SQL Database Researcher',
            goal='Generate an accurate SQL query based on the user''s natural language request.',
            backstory='''You are an expert SQL developer. You have been given access to a database
            and your sole purpose is to translate a user's plain English question into a
            perfect, executable SQL query.''',
            tools=[nl2sql],
            verbose=True,
            allow_delegation=False,
            llm=gemini_llm  # Pass the LLM to the agent
        )

        # 3. Create a Task for the Agent
        sql_task = Task(
            description=f"Generate a SQL query that answers this question: '{request.nl_query}'",
            expected_output="A single, valid, and executable SQL query string. Do not include any other text or explanation. Just the SQL query.",
            agent=sql_agent
        )

        # 4. Create and run the Crew
        crew = Crew(
            agents=[sql_agent],
            tasks=[sql_task],
            verbose=1
        )
        raw_result = crew.kickoff()

        # Extract final text from various possible result shapes
        def to_text(result):
            if isinstance(result, str):
                return result
            if isinstance(result, dict):
                for key in ("final_output", "result", "output", "content", "text"):
                    if key in result and isinstance(result[key], str):
                        return result[key]
            for attr in ("final_output", "result", "output", "content", "text", "raw"):
                if hasattr(result, attr):
                    val = getattr(result, attr)
                    if isinstance(val, str):
                        return val
            return str(result)

        sql_text = to_text(raw_result)

        # Strip common markdown fences if present
        if "```sql" in sql_text:
            try:
                sql_text = sql_text.split("```sql", 1)[1].split("```", 1)[0]
            except Exception:
                pass
        elif "```" in sql_text:
            try:
                sql_text = sql_text.split("```", 1)[1].split("```", 1)[0]
            except Exception:
                pass

        return PlainTextResponse(sql_text.strip(), media_type="text/plain; charset=utf-8")

    except Exception as e:
        print(f"Error processing request: {e}") 
        raise HTTPException(
            status_code=500, 
            detail=f"Failed to generate SQL: {str(e)}"
        )

# --- Root endpoint ---
@app.get("/")
async def root():
    return {"message": "Maxim NL-to-SQL API is running!"}

if __name__ == "__main__":
    uvicorn.run(app, host="127.0.0.1", port=5000)