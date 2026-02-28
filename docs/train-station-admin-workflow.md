# Train Station Admin Workflow (Local Dev)

This workflow keeps production bootstrap unchanged: production still seeds from committed JSON (`json/hkmtr.json`).

## Steps

1. Start backend and frontend locally.
2. Open admin station editor at `/admin/station`.
3. Upload a new map image if needed.
4. Edit stations:
   - click map to create a station draft
   - drag station pins to reposition
   - update station label/local name/coordinates
   - manage station line metadata
5. Save station and line changes.
6. Export JSON from admin editor.
7. Replace backend JSON file (`json/hkmtr.json`) with exported content.
8. Commit changes (JSON + any station map asset metadata/code changes) to Git.
9. Deploy as usual; startup seed path reads committed JSON and populates `train_stations`.

## Notes

- If map image upload fails, verify file type is an image and size is <= 10 MB.
- Exported JSON keeps seed-compatible shape: `version`, `type`, `name`, `data[]`.
