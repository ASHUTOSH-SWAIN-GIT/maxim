-- Database Backup for: test
-- Generated on: 2025-11-04 04:13:39
-- This file contains SQL statements to recreate the database schema from scratch

BEGIN;

CREATE TABLE "products" (
    "id" int4 NOT NULL DEFAULT nextval('products_id_seq'::regclass),
    "name" character varying(100) NOT NULL,
    "price" numeric(10,2),
    "stock_quantity" int4
, PRIMARY KEY ("id")
);

CREATE TABLE "test_log" (
    "id" int4 NOT NULL DEFAULT nextval('test_log_id_seq'::regclass),
    "message" text
, PRIMARY KEY ("id")
);

CREATE TABLE "users" (
    "id" int4 NOT NULL DEFAULT nextval('users_id_seq'::regclass),
    "username" character varying(50) NOT NULL,
    "email" character varying(100),
    "created_at" timestamp DEFAULT 'now()'
, PRIMARY KEY ("id")
);


CREATE UNIQUE INDEX users_email_key ON public.users USING btree (email);
CREATE SEQUENCE "products_id_seq" AS integer START 1 INCREMENT 1 MINVALUE 1 MAXVALUE 2147483647;
CREATE SEQUENCE "test_log_id_seq" AS integer START 1 INCREMENT 1 MINVALUE 1 MAXVALUE 2147483647;
CREATE SEQUENCE "users_id_seq" AS integer START 1 INCREMENT 1 MINVALUE 1 MAXVALUE 2147483647;


COMMIT;
