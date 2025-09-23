from pydantic import BaseModel
from typing import List, Any
from fastapi import FastAPI, UploadFile, HTTPException
from utils.pdf_parser import analyze_text_structure
import pymupdf
import os
import tempfile
app = FastAPI()

class StructuredContent(BaseModel):
    chapter: str
    content: str
    page: int
class ProcessResponse(BaseModel):
    paragraphs: List[StructuredContent]

class HealthResponse(BaseModel):
    status: str

@app.post("/process_pdf", response_model=ProcessResponse)
async def process_pdf(file: UploadFile):
    if not file.filename.lower().endswith(".pdf"):
        raise HTTPException(status_code=400, detail="Only PDF files supported")
    try:
        with tempfile.NamedTemporaryFile(delete=False, suffix=".pdf") as tmp:
            tmp.write(await file.read())
            tmp_path = tmp.name

        with pymupdf.open(tmp_path) as doc:
            result = analyze_text_structure(doc)

        return ProcessResponse(
            paragraphs=result
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Processing error: {e}")
    finally:
        if 'tmp_path' in locals() and os.path.exists(tmp_path):
            os.remove(tmp_path)


@app.get("/health")
def health():
    return {"status": "ok"}
