-- Seed: abstract root type 'entity'. Every object in the system is an Entity.
INSERT INTO types (slug, parent_id, concrete, name, description)
VALUES (
  'entity',
  NULL,
  false,
  'Entity',
  'Abstract root type. Every object in the system is an Entity.'
);
