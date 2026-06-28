import os

files = [
    "apps/frontend/src/routes/organizer/+page.svelte",
    "apps/frontend/src/routes/organizer/events/+page.ts",
    "apps/frontend/src/routes/organizer/events/[eventId]/dashboard/+page.svelte",
    "apps/frontend/src/routes/organizer/events/new/+page.svelte",
    "apps/frontend/src/routes/organizer/events/new/+page.ts",
    "apps/frontend/src/routes/organizer/venues/+page.ts"
]

for filepath in files:
    with open(filepath, 'r') as f:
        content = f.read()
    
    new_content = content.replace('/vendor/', '/organizer/')
    
    if content != new_content:
        with open(filepath, 'w') as f:
            f.write(new_content)
            print(f"Updated {filepath}")
