from utils import get_logger
import config
import traceback

logger = get_logger(__name__)
_embedding_model = None

def get_local_embedding_model():
    global _embedding_model

    if _embedding_model is None:
        try:
            from sentence_transformers import SentenceTransformer
            model_name = getattr(config, 'EMBEDDING_MODEL_NAME', 'paraphrase-multilingual-MiniLM-L12-v2')
            logger.info(f"Loading local embedding model: {model_name}")

            # 优化加载速度的配置
            _embedding_model = SentenceTransformer(
                model_name,
                device='cpu',  # 明确指定设备，避免自动检测开销
                cache_folder=None,  # 使用默认缓存路径
            )

            # 设置推理优化
            _embedding_model.eval()  # 设置为评估模式

            logger.info(f"✓ Model loaded successfully")
        except ImportError:
            error_msg = "sentence-transformers not installed. Install with: pip install sentence-transformers"
            logger.error(error_msg)
            logger.error(f"Traceback:\n{traceback.format_exc()}")
            raise ImportError(error_msg)
        except Exception as e:
            logger.error(f"Failed to load model: {e}")
            logger.error(f"Traceback:\n{traceback.format_exc()}")
            raise

    return _embedding_model


def vectorize(context: str): # type: ignore
    """Vectorize a single text string.

    Args:
        context: Text to vectorize

    Returns:
        List of floats representing the embedding vector
    """
    model = get_local_embedding_model()
    try:
        embeddings = model.encode(context).tolist()
        logger.debug(f"Generated embeddings for context, dimension: {len(embeddings)}")
        return embeddings
    except Exception as e:
        error_msg = f"Failed to generate embeddings: {e}"
        logger.error(error_msg)
        logger.error(f"Traceback:\n{traceback.format_exc()}")
        raise Exception(error_msg)
