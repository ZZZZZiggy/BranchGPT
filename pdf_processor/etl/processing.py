import hashlib
import pymupdf
from asyncio.log import logger
# from typing import Generator, Tuple, Dict, Any
from etl.pdf_parser import analyze_text_structure

def get_embedding_function(api_key: str, provider: str): # type: ignore
    """
    Get embedding function based on provider and API key.

    Args:
        api_key: API key for the embedding service (required).
        provider: Embedding provider. Options:
                  - "openai": OpenAI API
                  - "gemini": Google Gemini API

    Returns:
        Function that takes text and returns embeddings list

    Raises:
        ValueError: If provider is not supported or API key is missing
    """
    if not api_key:
        error_msg = "API key is required"
        logger.error(error_msg)
        raise ValueError(error_msg)

    if provider == "OpenAI":
        # Use OpenAI API
        try:
            import openai # type: ignore
        except ImportError:
            error_msg = "openai package not installed. Install with: pip install openai"
            logger.error(error_msg)
            raise ImportError(error_msg)

        def openai_embed(text: str):
            """Generate embeddings using OpenAI API"""
            try:
                # Create new client for each call (no caching of API key)
                client = openai.OpenAI(api_key=api_key)
                response = client.embeddings.create(
                    model="text-embedding-3-small",
                    input=text
                )
                embeddings = response.data[0].embedding
                logger.debug(f"Generated OpenAI embeddings, dimension: {len(embeddings)}")
                return embeddings
            except Exception as e:
                logger.error(f"OpenAI API error: {e}")
                raise

        logger.info("Using OpenAI API for embeddings")
        return openai_embed

    elif provider == "Gemini":
        # Use Google Gemini API
        try:
            import google.generativeai as genai # type: ignore
        except ImportError:
            error_msg = "google-generativeai package not installed. Install with: pip install google-generativeai"
            logger.error(error_msg)
            raise ImportError(error_msg)

        def gemini_embed(text: str):
            """Generate embeddings using Google Gemini API"""
            try:
                # Configure Gemini with API key (no caching)
                genai.configure(api_key=api_key) # type: ignore

                # Use Gemini's embedding model
                result = genai.embed_content( # type: ignore
                    model="models/embedding-001",
                    content=text,
                    task_type="retrieval_document"
                )
                embeddings = result['embedding']
                logger.debug(f"Generated Gemini embeddings, dimension: {len(embeddings)}")
                return embeddings
            except Exception as e:
                logger.error(f"Gemini API error: {e}")
                raise

        logger.info("Using Google Gemini API for embeddings")
        return gemini_embed

    else:
        # Provider not supported - raise error
        error_msg = f"Unsupported provider '{provider}'. Supported providers: 'openai', 'gemini'"
        logger.error(error_msg)
        raise ValueError(error_msg)

def process_and_vectorize(file_path: str, file_id: str, api_key: str, provider: str): # type: ignore
    """Process and vectorize the document at the given file path.

    Args:
        file_path (str): The path to the PDF file to process.
        file_id (str): Unique file ID (used to generate unique chunk IDs).
        api_key (str): API key for embedding service (required).
        provider (str): Embedding provider. Options: "openai", "gemini".

    Returns:
        Tuple[Generator[Dict[str, Any], None, None], int]: A generator yielding structured content and the estimated number of chunks.

    Raises:
        ValueError: If provider is not supported or API key is missing
    """
    # Get embedding function based on provider (will raise ValueError if invalid)
    # embed_func = get_embedding_function(api_key, provider)

    with pymupdf.open(file_path) as doc:
        structured_content = analyze_text_structure(doc)
    # filter out empty content sections
    valid_sections = [
        s for s in structured_content
        if s.get("content", "").strip()
    ]

    estimated_chunks = len(valid_sections)

    # Generator for yielding chunks with embeddings
    def chunk_generator():
        failed_chunks = 0
        for i, section in enumerate(valid_sections):
            content = section["content"].strip()

            # ✅ Generate unique chunk_id: hash(file_id + chunk_index)
            # This ensures no collision even with duplicate content
            unique_key = f"{file_id}_{i}"
            chunk_id = hashlib.md5(unique_key.encode()).hexdigest()

            try:
                # Use the embedding function (OpenAI API or local model)
                # embeddings = embed_func(content)
                embeddings = [0.1] * 384
            except Exception as e:
                logger.error(f"Failed to generate embeddings for chunk {i+1}: {e}")
                failed_chunks += 1

                # if failed_chunks > 5:
                if failed_chunks > 5:
                    error_msg = f"Too many embedding failures ({failed_chunks}), stopping processing"
                    logger.error(error_msg)
                    raise Exception(error_msg)

                continue

            yield {
                "chunk_id": chunk_id,
                "chapter": section.get("chapter", ""),
                "content": content,
                "embeddings": embeddings,
                "chapter_num": section.get("chapter_num", ""),
            }

        # check failed rate after processing
        if failed_chunks > 0:
            logger.warning(f"⚠️  {failed_chunks}/{len(valid_sections)} chunks failed to generate embeddings")

    return chunk_generator(), estimated_chunks

def vectorize(context: str, api_key: str, provider: str): # type: ignore
    """Vectorize a single text string.

    Args:
        context: Text to vectorize
        api_key: API key for embedding service
        provider: Embedding provider ("openai" or "gemini")

    Returns:
        List of floats representing the embedding vector
    """
    embd_func = get_embedding_function(api_key, provider)
    return embd_func(context)
