from fastapi import FastAPI

from appboard.models import Row

app = FastAPI()


@app.get("/rows/{row_id}")
def get_row(row_id: str) -> Row:
    return Row(id=row_id, title="demo")
