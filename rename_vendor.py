import os
import re

def replace_case_insensitive(text, search, replace):
    # This is a basic case-preserving replacement
    # We will replace Vendor -> Organizer, vendor -> organizer
    text = text.replace('Vendor', 'Organizer')
    text = text.replace('vendor', 'organizer')
    text = text.replace('VENDOR', 'ORGANIZER')
    return text

directory = 'apps/apigateway/api'
for filename in os.listdir(directory):
    if filename.endswith('.go'):
        filepath = os.path.join(directory, filename)
        with open(filepath, 'r') as f:
            content = f.read()
        
        new_content = replace_case_insensitive(content, 'vendor', 'organizer')
        
        if content != new_content:
            with open(filepath, 'w') as f:
                f.write(new_content)
                print(f"Updated {filepath}")
