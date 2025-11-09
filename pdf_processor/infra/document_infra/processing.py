import hashlib
import pymupdf
import traceback
from utils import get_logger
import config
from infra.document_infra.pdf_parser import analyze_text_structure
from infra.document_infra.embedding import get_local_embedding_model

logger = get_logger(__name__)



def process_and_vectorize(file_path: str, file_id: str, rag_mode: bool): # type: ignore
    """Process and vectorize the document at the given file path using local model.

    Args:
        file_path (str): The path to the PDF file to process.
        file_id (str): Unique file ID (used to generate unique chunk IDs).

    Returns:
        Tuple[Generator[Dict[str, Any], None, None], int]: A generator yielding structured content and the estimated number of chunks.
    """
    if rag_mode:
        model = get_local_embedding_model()

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

        if not rag_mode:
            # zero embeddings for non-RAG mode
            zero_embedding = [0.0] * 384

            for i, section in enumerate(valid_sections):
                unique_key = f"{file_id}_{i}"
                chunk_id = hashlib.md5(unique_key.encode()).hexdigest()

                yield {
                    "chunk_id": chunk_id,
                    "chapter": section.get("chapter", ""),
                    "content": section["content"].strip(),
                    "embeddings": zero_embedding,
                    "chapter_num": section.get("chapter_num", ""),
                }
            return

        batch_size = 8  # batch size for embedding generation

        # 批量处理 embeddings
        for batch_start in range(0, len(valid_sections), batch_size):
            batch_end = min(batch_start + batch_size, len(valid_sections))
            batch_sections = valid_sections[batch_start:batch_end]

            # 准备批量文本
            batch_texts = [s["content"].strip() for s in batch_sections]

            try:
                # 批量生成 embeddings（并行处理，速度提升 2-4倍）
                batch_embeddings = model.encode(
                    batch_texts,
                    batch_size=batch_size,
                    show_progress_bar=False,
                    convert_to_tensor=False,  # 返回 numpy array，更容易迭代
                    convert_to_numpy=True
                )
                logger.debug(f"Generated embeddings for batch {batch_start//batch_size + 1}, chunks {batch_start+1}-{batch_end}")

                # 逐个 yield 结果
                for i, section in enumerate(batch_sections):
                    embeddings = batch_embeddings[i]  # 直接索引而不是 zip
                    global_idx = batch_start + i
                    unique_key = f"{file_id}_{global_idx}"
                    chunk_id = hashlib.md5(unique_key.encode()).hexdigest()

                    yield {
                        "chunk_id": chunk_id,
                        "chapter": section.get("chapter", ""),
                        "content": section["content"].strip(),
                        "embeddings": embeddings.tolist(),
                        "chapter_num": section.get("chapter_num", ""),
                    }

            except Exception as e:
                logger.error(f"Failed to generate embeddings for batch {batch_start//batch_size + 1}: {e}")
                logger.error(f"Traceback:\n{traceback.format_exc()}")

                # 批处理失败，回退到逐个处理
                logger.warning(f"Falling back to sequential processing for batch {batch_start//batch_size + 1}")
                for i, section in enumerate(batch_sections):
                    global_idx = batch_start + i
                    content = section["content"].strip()
                    unique_key = f"{file_id}_{global_idx}"
                    chunk_id = hashlib.md5(unique_key.encode()).hexdigest()

                    try:
                        embeddings = model.encode(content).tolist()
                        yield {
                            "chunk_id": chunk_id,
                            "chapter": section.get("chapter", ""),
                            "content": content,
                            "embeddings": embeddings,
                            "chapter_num": section.get("chapter_num", ""),
                        }
                    except Exception as e:
                        logger.error(f"Failed to generate embeddings for chunk {global_idx+1}: {e}")
                        failed_chunks += 1
                        if failed_chunks > 5:
                            error_msg = f"Too many embedding failures ({failed_chunks}), stopping processing"
                            logger.error(error_msg)
                            raise Exception(error_msg)
                        continue

        # check failed rate after processing
        if failed_chunks > 0:
            logger.warning(f"{failed_chunks}/{len(valid_sections)} chunks failed to generate embeddings")

    return chunk_generator(), estimated_chunks
