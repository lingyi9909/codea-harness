import importlib.util
from pathlib import Path
import unittest

MODULE_PATH=Path(__file__).resolve().parents[1]/'scripts/validate_package.py'
spec=importlib.util.spec_from_file_location('validate_package', MODULE_PATH)
module=importlib.util.module_from_spec(spec); spec.loader.exec_module(module)

class PackageValidationTest(unittest.TestCase):
    def test_required_files_exist(self): module.validate_required()
    def test_contract_schemas_are_valid(self): module.validate_schemas()
    def test_example_yaml_is_safe(self): module.validate_yaml()

if __name__ == '__main__': unittest.main()
