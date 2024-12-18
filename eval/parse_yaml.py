import yaml
import sys

def load_yaml(file_path):
    with open(file_path, 'r') as f:
        return yaml.safe_load(f)

if __name__ == '__main__':
    config_file = sys.argv[1]
    query = sys.argv[2]

    data = load_yaml(config_file)
    if query == 'tasks':
        for task in data['tasks']:
            print(task['name'])
    elif query == 'metaprompts':
        for prompt in data['metaprompts']:
            print(prompt['prompt'])
