CREATE TABLE accounts (
    owner_uuid uuid,
    aws_access_key_id character varying,
    aws_secret_access_key_token bytea
);


CREATE TABLE addon_resources (
    owner_uuid uuid,
    provider_resource_id character varying,
    heroku_resource_id character varying,
    deleted_at timestamp without time zone,
    mark_for_deletion boolean DEFAULT false,
    aws_access_key_id character varying
);
