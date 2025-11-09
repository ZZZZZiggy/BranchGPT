"""
Unified logging configuration for PDF Processor.

Provides a standardized logger with:
- Console output with colors
- File output with rotation
- Consistent formatting across all modules
"""

import logging
import sys
from pathlib import Path
from logging.handlers import RotatingFileHandler
from typing import Optional
import config


def setup_logger(name: Optional[str] = None) -> logging.Logger:
    """
    Get or create a configured logger instance.

    Args:
        name: Logger name (typically __name__ from calling module)
              If None, returns root logger

    Returns:
        Configured logger instance

    Example:
        >>> from utils.logger import setup_logger
        >>> logger = setup_logger(__name__)
        >>> logger.info("Processing started")
    """
    logger = logging.getLogger(name)

    # Only configure if not already configured
    if logger.handlers:
        return logger

    # Set level from config
    log_level = getattr(config, 'LOG_LEVEL', 'INFO')
    logger.setLevel(getattr(logging, log_level.upper(), logging.INFO))

    # Console handler with colors
    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setLevel(logging.DEBUG)

    # Console format: simpler for readability
    console_format = logging.Formatter(
        '%(levelname)s [%(name)s] %(message)s'
    )
    console_handler.setFormatter(console_format)

    # File handler with rotation
    log_file = getattr(config, 'LOG_FILE', 'logs/app.log')
    log_path = Path(log_file)
    log_path.parent.mkdir(parents=True, exist_ok=True)

    file_handler = RotatingFileHandler(
        log_file,
        maxBytes=10 * 1024 * 1024,  # 10MB
        backupCount=5,
        encoding='utf-8'
    )
    file_handler.setLevel(logging.DEBUG)

    # File format: more detailed with timestamp
    file_format = logging.Formatter(
        '%(asctime)s | %(levelname)-8s | %(name)s | %(funcName)s:%(lineno)d | %(message)s',
        datefmt='%Y-%m-%d %H:%M:%S'
    )
    file_handler.setFormatter(file_format)

    # Add handlers
    logger.addHandler(console_handler)
    logger.addHandler(file_handler)

    # Prevent propagation to avoid duplicate logs
    logger.propagate = False

    return logger


# Convenience function for quick logger access
def get_logger(name: Optional[str] = None) -> logging.Logger:
    """
    Alias for setup_logger for convenience.

    Args:
        name: Logger name (typically __name__)

    Returns:
        Configured logger instance
    """
    return setup_logger(name)


# Pre-configured root logger for the application
app_logger = setup_logger('pdf_processor')
