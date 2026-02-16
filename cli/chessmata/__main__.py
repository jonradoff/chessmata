"""Allow running as python -m chessmata."""

from .main import main
import sys

if __name__ == '__main__':
    sys.exit(main())
