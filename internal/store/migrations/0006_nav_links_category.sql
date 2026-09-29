-- Categorise quick-nav links so they can be grouped under a hover submenu.
-- Empty category = shown directly in the top bar (as before).
ALTER TABLE nav_links ADD COLUMN category TEXT NOT NULL DEFAULT '';
