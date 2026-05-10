Learnings

- TagFilter originally supported multi-select by storing selectedTagIds in a Set and adding/removing.
- Backend ListFilter.tag is a single string used in SQL 'WHERE tag_id = $1' so multi-select must be translated to single tag on frontend.
- Chose single-select in TagFilter for better UX and clarity; LibraryPage also updated to use first tag if multiple provided.
- Kept onFilterChange API as (tagIds: string[]) for backward compatibility; now guarantees 0 or 1 ids.
