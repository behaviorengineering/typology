from pydantic import BaseModel


class Row(BaseModel):
    id: str
    title: str
