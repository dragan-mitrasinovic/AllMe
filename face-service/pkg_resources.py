"""Stand-in for setuptools' pkg_resources, which setuptools 84+ no longer ships.

face_recognition_models only calls resource_filename() to locate its bundled model files.
"""
from importlib.resources import files


def resource_filename(package: str, resource: str) -> str:
    return str(files(package).joinpath(resource))
