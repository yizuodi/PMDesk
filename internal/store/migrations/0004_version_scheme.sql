-- Structured version-number scheme support.
-- Versions are entered as a free-text string (e.g. R027C10B003, SPC200B020)
-- and parsed into a type + numeric components for validation, filtering and
-- R→C grouping. Legacy / non-conforming strings keep vtype = ''.
--
--   C  version  R###C##        e.g. R027C10      (half-year container)
--   B  build    R###C##B###    e.g. R027C10B003  (internal dev build)
--   SPC         SPC###B###     e.g. SPC200B020   (pre-release, belongs to a C)
--   CP          CP####B###     e.g. CP1022B020   (pre-release, belongs to a C)
ALTER TABLE versions ADD COLUMN vtype     TEXT    NOT NULL DEFAULT '';
ALTER TABLE versions ADD COLUMN r_num     INTEGER;  -- R version number (C, B; SPC/CP inherit from parent C)
ALTER TABLE versions ADD COLUMN c_num     INTEGER;  -- C version number
ALTER TABLE versions ADD COLUMN b_num     INTEGER;  -- B build number
ALTER TABLE versions ADD COLUMN extra_num INTEGER;  -- SPC/CP sequence (the 200 / 1022 part)
