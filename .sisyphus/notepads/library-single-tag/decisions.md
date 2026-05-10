Decisions for making TagFilter single-select

- Chosen Option B: Make TagFilter single-select so frontend sends a single tag ID to the backend's ListFilter.tag field.
- Rationale: Backend expects a single tag ID. Changing API or backend is out of scope. Single-select provides consistent UX (active tag).
- Implementation: Toggle logic changed to ensure selectedTagIds contains 0 or 1 id. LibraryPage updated to use tagIds[0] when building filter.
- Verified: tsc build passes and vite build completes.
