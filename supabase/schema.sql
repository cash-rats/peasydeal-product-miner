


SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE EXTENSION IF NOT EXISTS "pg_net" WITH SCHEMA "extensions";






COMMENT ON SCHEMA "public" IS 'standard public schema';



CREATE EXTENSION IF NOT EXISTS "pg_graphql" WITH SCHEMA "graphql";






CREATE EXTENSION IF NOT EXISTS "pg_stat_statements" WITH SCHEMA "extensions";






CREATE EXTENSION IF NOT EXISTS "pg_trgm" WITH SCHEMA "public";






CREATE EXTENSION IF NOT EXISTS "pgcrypto" WITH SCHEMA "extensions";






CREATE EXTENSION IF NOT EXISTS "supabase_vault" WITH SCHEMA "vault";






CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA "extensions";






CREATE TYPE "public"."cancel_reason" AS ENUM (
    'fraud',
    'customer',
    'declined',
    'inventory',
    'other',
    'none'
);


ALTER TYPE "public"."cancel_reason" OWNER TO "postgres";


CREATE TYPE "public"."category_type" AS ENUM (
    'category',
    'promotion',
    'seasonal',
    'taxonomy_category'
);


ALTER TYPE "public"."category_type" OWNER TO "postgres";


CREATE TYPE "public"."discount_type" AS ENUM (
    'price_off',
    'percentage_off',
    'free_shipping',
    'superdeal_percentage_off',
    'email_subscribe'
);


ALTER TYPE "public"."discount_type" OWNER TO "postgres";


CREATE TYPE "public"."order_status" AS ENUM (
    'order_received',
    'processing',
    'hold',
    'complete',
    'cancelled'
);


ALTER TYPE "public"."order_status" OWNER TO "postgres";


CREATE TYPE "public"."payment_method" AS ENUM (
    'stripe',
    'paypal'
);


ALTER TYPE "public"."payment_method" OWNER TO "postgres";


CREATE TYPE "public"."payment_status" AS ENUM (
    'unpaid',
    'paid',
    'review_refund',
    'refunded'
);


ALTER TYPE "public"."payment_status" OWNER TO "postgres";


CREATE TYPE "public"."price_type" AS ENUM (
    'default',
    'sales',
    'default_price'
);


ALTER TYPE "public"."price_type" OWNER TO "postgres";


CREATE TYPE "public"."shipping_status" AS ENUM (
    'pending',
    'shipping',
    'unshipped',
    'shipped'
);


ALTER TYPE "public"."shipping_status" OWNER TO "postgres";

SET default_tablespace = '';

SET default_table_access_method = "heap";


CREATE TABLE IF NOT EXISTS "public"."uk_addresses" (
    "id" bigint NOT NULL,
    "line_1" character varying(255),
    "line_2" character varying(255),
    "line_3" character varying(255),
    "post_town" character varying(255),
    "county" character varying(255),
    "postcode" character varying(10),
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone,
    "postcode_norm" "text" GENERATED ALWAYS AS (
CASE
    WHEN ("postcode" IS NULL) THEN NULL::"text"
    ELSE "upper"("regexp_replace"(("postcode")::"text", '\s+'::"text", ''::"text", 'g'::"text"))
END) STORED
);


ALTER TABLE "public"."uk_addresses" OWNER TO "postgres";


CREATE OR REPLACE FUNCTION "public"."search_uk_addresses_by_postcode"("p_input" "text", "p_limit" integer DEFAULT 10) RETURNS SETOF "public"."uk_addresses"
    LANGUAGE "sql" STABLE
    AS $$
    WITH input AS (
        SELECT
            upper(regexp_replace(coalesce(p_input, ''), '\s+', '', 'g')) AS q,
            LEAST(GREATEST(coalesce(p_limit, 10), 1), 50) AS lim
    )
    SELECT ua.*
    FROM public.uk_addresses ua
    WHERE ua.deleted_at IS NULL
      AND ua.postcode_norm IS NOT NULL
      AND (SELECT q FROM input) <> ''
      AND (
          (length((SELECT q FROM input)) < 3 AND ua.postcode_norm LIKE (SELECT q FROM input) || '%')
          OR (length((SELECT q FROM input)) >= 3 AND ua.postcode_norm % (SELECT q FROM input))
      )
    ORDER BY similarity(ua.postcode_norm, (SELECT q FROM input)) DESC, ua.id
    LIMIT (SELECT lim FROM input);
$$;


ALTER FUNCTION "public"."search_uk_addresses_by_postcode"("p_input" "text", "p_limit" integer) OWNER TO "postgres";


COMMENT ON FUNCTION "public"."search_uk_addresses_by_postcode"("p_input" "text", "p_limit" integer) IS 'Fuzzy search for UK addresses by postcode using trigram similarity on normalized postcodes.';



CREATE TABLE IF NOT EXISTS "public"."categories" (
    "id" integer NOT NULL,
    "category_id" integer,
    "label" "text",
    "url" "text",
    "icon_url" "text",
    "order_index" integer,
    "created_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone,
    "enable" boolean DEFAULT true,
    "show" boolean DEFAULT true,
    "type" "public"."category_type",
    "name" "text",
    "description" "text",
    "parent_id" integer,
    "tier" integer,
    "short_name" "text",
    "equiv_taxonomy_name" character varying(30)
);


ALTER TABLE "public"."categories" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."categories_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."categories_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."categories_id_seq" OWNED BY "public"."categories"."id";



CREATE TABLE IF NOT EXISTS "public"."category_products" (
    "id" integer NOT NULL,
    "category_id" integer NOT NULL,
    "product_id" integer NOT NULL,
    "is_main" "text",
    "created_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone
);


ALTER TABLE "public"."category_products" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."category_products_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."category_products_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."category_products_id_seq" OWNED BY "public"."category_products"."id";



CREATE TABLE IF NOT EXISTS "public"."customer_info" (
    "id" bigint NOT NULL,
    "email" character varying(254),
    "uuid" character varying(254),
    "name" character varying(254),
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone,
    "is_valid" boolean
);


ALTER TABLE "public"."customer_info" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."customer_info_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."customer_info_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."customer_info_id_seq" OWNED BY "public"."customer_info"."id";



CREATE TABLE IF NOT EXISTS "public"."discount_codes" (
    "id" bigint NOT NULL,
    "discount_type" "public"."discount_type" DEFAULT 'price_off'::"public"."discount_type",
    "discount_code" "text",
    "valid_from" timestamp without time zone,
    "valid_to" timestamp without time zone,
    "redeemed_at" timestamp without time zone,
    "price_off_amount" numeric(10,2),
    "percentage_off_amount" numeric(10,2),
    "allow_usage_times" integer DEFAULT 1,
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone,
    "use_count" integer DEFAULT 0,
    "infinite" boolean DEFAULT false,
    "category_id" integer,
    "discount_message" "text",
    "email_list_id" integer,
    "amount_eq_above" numeric(10,2)
);


ALTER TABLE "public"."discount_codes" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."discount_codes_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."discount_codes_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."discount_codes_id_seq" OWNED BY "public"."discount_codes"."id";



CREATE TABLE IF NOT EXISTS "public"."edm_activity_emails" (
    "id" bigint NOT NULL,
    "customer_info_id" integer,
    "edm_recepient_list_id" character varying(255),
    "edm_activity_id" character varying(255),
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone
);


ALTER TABLE "public"."edm_activity_emails" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."edm_activity_emails_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."edm_activity_emails_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."edm_activity_emails_id_seq" OWNED BY "public"."edm_activity_emails"."id";



CREATE TABLE IF NOT EXISTS "public"."edm_activity_recipient" (
    "id" bigint NOT NULL,
    "edm_activity_recipient" integer,
    "edm_recipient_list_id" character varying(255),
    "edm_activity_id" character varying,
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone
);


ALTER TABLE "public"."edm_activity_recipient" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."edm_activity_recipient_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."edm_activity_recipient_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."edm_activity_recipient_id_seq" OWNED BY "public"."edm_activity_recipient"."id";



CREATE TABLE IF NOT EXISTS "public"."email_list" (
    "id" bigint NOT NULL,
    "email" character varying(254),
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone,
    "activate" boolean DEFAULT false,
    "uuid" character varying(255)
);


ALTER TABLE "public"."email_list" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."email_list_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."email_list_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."email_list_id_seq" OWNED BY "public"."email_list"."id";



CREATE TABLE IF NOT EXISTS "public"."gcs_list" (
    "id" integer NOT NULL,
    "url" character varying
);


ALTER TABLE "public"."gcs_list" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."gcs_list_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."gcs_list_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."gcs_list_id_seq" OWNED BY "public"."gcs_list"."id";



CREATE TABLE IF NOT EXISTS "public"."google_product_taxonomy" (
    "id" integer NOT NULL,
    "tier1" character varying,
    "tier2" character varying,
    "tier3" character varying,
    "tier4" character varying,
    "tier5" character varying,
    "tier6" character varying,
    "enable" boolean DEFAULT true
);


ALTER TABLE "public"."google_product_taxonomy" OWNER TO "postgres";


CREATE TABLE IF NOT EXISTS "public"."inventory" (
    "id" bigint NOT NULL,
    "total_stock" integer,
    "locked_stock" integer,
    "available_stock" integer,
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone,
    CONSTRAINT "inventory_available_stock_check" CHECK (("available_stock" > 0)),
    CONSTRAINT "inventory_locked_stock_check" CHECK (("locked_stock" > 0)),
    CONSTRAINT "inventory_total_stock_check" CHECK (("total_stock" > 0))
);


ALTER TABLE "public"."inventory" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."inventory_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."inventory_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."inventory_id_seq" OWNED BY "public"."inventory"."id";



CREATE TABLE IF NOT EXISTS "public"."order_items" (
    "id" bigint NOT NULL,
    "order_id" integer NOT NULL,
    "product_id" integer NOT NULL,
    "variation_id" integer NOT NULL,
    "order_quantity" integer,
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone,
    "carrier" "text",
    "tracking_number" "text",
    "has_send_email" boolean DEFAULT false,
    "shipping_agent_order_id" character varying(100),
    "can_review" boolean DEFAULT false,
    "reviewed" boolean DEFAULT false
);


ALTER TABLE "public"."order_items" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."order_items_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."order_items_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."order_items_id_seq" OWNED BY "public"."order_items"."id";



CREATE TABLE IF NOT EXISTS "public"."orders" (
    "id" bigint NOT NULL,
    "uuid" "text" NOT NULL,
    "order_status" "public"."order_status",
    "shipping_status" "public"."shipping_status",
    "payment_status" "public"."payment_status",
    "total_amount" numeric(10,2),
    "shipping_fee" numeric(10,2),
    "tax_amount" numeric(10,2),
    "currency" character varying(10),
    "discount_amount" numeric(10,2),
    "first_name" "text",
    "last_name" "text",
    "address" "text",
    "address2" "text",
    "address3" "text",
    "city" "text",
    "county" "text",
    "postalcode" "text",
    "country" "text",
    "tel" "text",
    "email" "text",
    "gift_message" "text",
    "customer_id" "text",
    "customer_client_device" "text",
    "cancel_reason" "public"."cancel_reason" DEFAULT 'none'::"public"."cancel_reason",
    "cancel_reason_other_text" "text",
    "cancelled_at" timestamp without time zone,
    "payment_token" "text",
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone,
    "contact_name" "text",
    "phone" "text",
    "subtotal" numeric(10,2),
    "promo_code" character varying(40),
    "payment_method" "public"."payment_method",
    "paypal_order_id" "text",
    "note" "text"
);


ALTER TABLE "public"."orders" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."orders_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."orders_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."orders_id_seq" OWNED BY "public"."orders"."id";



CREATE TABLE IF NOT EXISTS "public"."product_edit_records" (
    "id" integer NOT NULL,
    "product_id" bigint NOT NULL,
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "editor" character varying NOT NULL,
    "origin_json" "text"
);


ALTER TABLE "public"."product_edit_records" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."product_edit_records_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."product_edit_records_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."product_edit_records_id_seq" OWNED BY "public"."product_edit_records"."id";



CREATE TABLE IF NOT EXISTS "public"."product_images" (
    "variation_id" integer,
    "url" "text" NOT NULL,
    "created_at" timestamp without time zone DEFAULT "now"(),
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone,
    "product_id" bigint,
    "position" integer,
    "product_uuid" "text",
    "enable" boolean,
    "id" bigint NOT NULL
);


ALTER TABLE "public"."product_images" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."product_images_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."product_images_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."product_images_id_seq" OWNED BY "public"."product_images"."id";



CREATE TABLE IF NOT EXISTS "public"."product_order_statistics" (
    "id" bigint NOT NULL,
    "product_id" integer,
    "order_count" integer DEFAULT 0,
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone
);


ALTER TABLE "public"."product_order_statistics" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."product_order_statistics_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."product_order_statistics_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."product_order_statistics_id_seq" OWNED BY "public"."product_order_statistics"."id";



CREATE TABLE IF NOT EXISTS "public"."product_price_rule" (
    "id" bigint NOT NULL,
    "uuid" character varying(255) NOT NULL,
    "variation_id" integer,
    "enable" boolean DEFAULT true,
    "start_sale_date" timestamp without time zone,
    "end_sale_date" timestamp without time zone,
    "retail_price" numeric(10,2),
    "sale_price" numeric(10,2),
    "shipping_fee" numeric(10,2),
    "tax_rate" numeric(10,2),
    "currency" character varying(10),
    "price_type" "public"."price_type",
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone,
    "variation_uuid" "text"
);


ALTER TABLE "public"."product_price_rule" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."product_price_rule_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."product_price_rule_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."product_price_rule_id_seq" OWNED BY "public"."product_price_rule"."id";



CREATE TABLE IF NOT EXISTS "public"."product_ratings" (
    "id" bigint NOT NULL,
    "product_id" integer,
    "review_content" "text",
    "rating" numeric(3,1),
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone,
    "order_item_id" integer,
    "review_images" "text",
    "name" character varying(255),
    "masked_name" character varying(255)
);


ALTER TABLE "public"."product_ratings" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."product_ratings_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."product_ratings_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."product_ratings_id_seq" OWNED BY "public"."product_ratings"."id";



CREATE TABLE IF NOT EXISTS "public"."product_variations" (
    "id" bigint NOT NULL,
    "uuid" character varying(255) NOT NULL,
    "product_id" integer,
    "sku" "text",
    "spec_name" "text",
    "spec_content" "text",
    "short_description" "text",
    "enable" boolean DEFAULT true,
    "inventory_id" integer,
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone,
    "purchase_limit" integer,
    "shipping_weight_kg" numeric(10,3),
    "actual_weight_kg" numeric(10,3)
);


ALTER TABLE "public"."product_variations" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."product_variations_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."product_variations_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."product_variations_id_seq" OWNED BY "public"."product_variations"."id";



CREATE TABLE IF NOT EXISTS "public"."products" (
    "id" bigint NOT NULL,
    "uuid" "text" NOT NULL,
    "title" "text",
    "subtitle" "text",
    "description" "text",
    "enable" boolean DEFAULT true,
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone,
    "tag_combo_type" character varying(40),
    "seo_description" "text",
    "seo_title" "text",
    "weekly_deal_from" timestamp without time zone,
    "main_pic_id" integer,
    "source" character varying(255)
);


ALTER TABLE "public"."products" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."products_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."products_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."products_id_seq" OWNED BY "public"."products"."id";



CREATE SEQUENCE IF NOT EXISTS "public"."uk_addresses_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."uk_addresses_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."uk_addresses_id_seq" OWNED BY "public"."uk_addresses"."id";



CREATE TABLE IF NOT EXISTS "public"."users" (
    "id" bigint NOT NULL,
    "is_cms_user" boolean DEFAULT false,
    "enable" boolean DEFAULT false,
    "full_name" character varying(20),
    "name" character varying(20),
    "username" character varying(30),
    "password" character varying(30),
    "token" character varying(100),
    "token_expire_date" timestamp without time zone,
    "privilege" character varying(11) DEFAULT NULL::character varying,
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp without time zone
);


ALTER TABLE "public"."users" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."users_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE "public"."users_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."users_id_seq" OWNED BY "public"."users"."id";



ALTER TABLE ONLY "public"."categories" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."categories_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."category_products" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."category_products_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."customer_info" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."customer_info_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."discount_codes" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."discount_codes_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."edm_activity_emails" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."edm_activity_emails_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."edm_activity_recipient" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."edm_activity_recipient_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."email_list" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."email_list_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."gcs_list" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."gcs_list_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."inventory" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."inventory_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."order_items" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."order_items_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."orders" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."orders_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."product_edit_records" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."product_edit_records_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."product_images" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."product_images_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."product_order_statistics" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."product_order_statistics_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."product_price_rule" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."product_price_rule_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."product_ratings" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."product_ratings_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."product_variations" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."product_variations_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."products" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."products_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."uk_addresses" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."uk_addresses_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."users" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."users_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."categories"
    ADD CONSTRAINT "categories_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."category_products"
    ADD CONSTRAINT "category_products_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."customer_info"
    ADD CONSTRAINT "customer_info_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."discount_codes"
    ADD CONSTRAINT "discount_codes_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."edm_activity_emails"
    ADD CONSTRAINT "edm_activity_emails_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."edm_activity_recipient"
    ADD CONSTRAINT "edm_activity_recipient_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."email_list"
    ADD CONSTRAINT "email_list_email_key" UNIQUE ("email");



ALTER TABLE ONLY "public"."email_list"
    ADD CONSTRAINT "email_list_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."gcs_list"
    ADD CONSTRAINT "gcs_list_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."google_product_taxonomy"
    ADD CONSTRAINT "google_product_taxonomy_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."inventory"
    ADD CONSTRAINT "inventory_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."order_items"
    ADD CONSTRAINT "order_items_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."orders"
    ADD CONSTRAINT "orders_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."product_edit_records"
    ADD CONSTRAINT "product_edit_records_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."product_images"
    ADD CONSTRAINT "product_images_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."product_order_statistics"
    ADD CONSTRAINT "product_order_statistics_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."product_price_rule"
    ADD CONSTRAINT "product_price_rule_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."product_ratings"
    ADD CONSTRAINT "product_ratings_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."product_variations"
    ADD CONSTRAINT "product_variations_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."products"
    ADD CONSTRAINT "products_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."uk_addresses"
    ADD CONSTRAINT "uk_addresses_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."product_price_rule"
    ADD CONSTRAINT "unique_price_rule_uuid" UNIQUE ("uuid");



ALTER TABLE ONLY "public"."category_products"
    ADD CONSTRAINT "unique_product_category" UNIQUE ("product_id", "category_id");



ALTER TABLE ONLY "public"."products"
    ADD CONSTRAINT "unique_uuid" UNIQUE ("uuid");



ALTER TABLE ONLY "public"."product_variations"
    ADD CONSTRAINT "unique_variation_uuid" UNIQUE ("uuid");



ALTER TABLE ONLY "public"."users"
    ADD CONSTRAINT "users_pkey" PRIMARY KEY ("id");



CREATE INDEX "uk_addresses_postcode_norm_trgm_idx" ON "public"."uk_addresses" USING "gin" ("postcode_norm" "public"."gin_trgm_ops");



ALTER TABLE ONLY "public"."categories"
    ADD CONSTRAINT "categories_category_id_fkey" FOREIGN KEY ("category_id") REFERENCES "public"."categories"("id") ON DELETE SET NULL;



COMMENT ON CONSTRAINT "categories_category_id_fkey" ON "public"."categories" IS 'Self-referential foreign key for category hierarchy. Sets to NULL when parent category is deleted.';



ALTER TABLE ONLY "public"."categories"
    ADD CONSTRAINT "categories_parent_id_fkey" FOREIGN KEY ("parent_id") REFERENCES "public"."categories"("id") ON DELETE SET NULL;



ALTER TABLE ONLY "public"."category_products"
    ADD CONSTRAINT "category_products_category_id_fkey" FOREIGN KEY ("category_id") REFERENCES "public"."categories"("id") ON DELETE CASCADE;



COMMENT ON CONSTRAINT "category_products_category_id_fkey" ON "public"."category_products" IS 'Foreign key constraint ensuring category association references valid category. Cascades deletion when category is deleted.';



ALTER TABLE ONLY "public"."category_products"
    ADD CONSTRAINT "category_products_product_id_fkey" FOREIGN KEY ("product_id") REFERENCES "public"."products"("id") ON DELETE CASCADE;



COMMENT ON CONSTRAINT "category_products_product_id_fkey" ON "public"."category_products" IS 'Foreign key constraint ensuring category association references valid product. Cascades deletion when product is deleted.';



ALTER TABLE ONLY "public"."discount_codes"
    ADD CONSTRAINT "discount_codes_category_id_fkey" FOREIGN KEY ("category_id") REFERENCES "public"."categories"("id") ON DELETE SET NULL;



COMMENT ON CONSTRAINT "discount_codes_category_id_fkey" ON "public"."discount_codes" IS 'Foreign key constraint linking discounts to categories. Sets to NULL when category is deleted, making discount site-wide.';



ALTER TABLE ONLY "public"."discount_codes"
    ADD CONSTRAINT "discount_codes_email_list_id_fkey" FOREIGN KEY ("email_list_id") REFERENCES "public"."email_list"("id") ON DELETE SET NULL;



COMMENT ON CONSTRAINT "discount_codes_email_list_id_fkey" ON "public"."discount_codes" IS 'Foreign key constraint linking discounts to email lists. Sets to NULL when email list is deleted.';



ALTER TABLE ONLY "public"."edm_activity_emails"
    ADD CONSTRAINT "edm_activity_emails_customer_info_id_fkey" FOREIGN KEY ("customer_info_id") REFERENCES "public"."customer_info"("id") ON DELETE SET NULL;



COMMENT ON CONSTRAINT "edm_activity_emails_customer_info_id_fkey" ON "public"."edm_activity_emails" IS 'Foreign key constraint linking EDM activities to customers. Sets to NULL when customer is deleted, preserving activity log.';



ALTER TABLE ONLY "public"."order_items"
    ADD CONSTRAINT "order_items_order_id_fkey" FOREIGN KEY ("order_id") REFERENCES "public"."orders"("id") ON DELETE RESTRICT;



COMMENT ON CONSTRAINT "order_items_order_id_fkey" ON "public"."order_items" IS 'Foreign key constraint ensuring order_id references a valid order. Prevents deletion of orders with existing order_items.';



ALTER TABLE ONLY "public"."order_items"
    ADD CONSTRAINT "order_items_product_id_fkey" FOREIGN KEY ("product_id") REFERENCES "public"."products"("id") ON DELETE RESTRICT;



COMMENT ON CONSTRAINT "order_items_product_id_fkey" ON "public"."order_items" IS 'Foreign key constraint ensuring product_id references a valid product. Prevents deletion of products with existing order_items.';



ALTER TABLE ONLY "public"."order_items"
    ADD CONSTRAINT "order_items_variation_id_fkey" FOREIGN KEY ("variation_id") REFERENCES "public"."product_variations"("id") ON DELETE RESTRICT;



COMMENT ON CONSTRAINT "order_items_variation_id_fkey" ON "public"."order_items" IS 'Foreign key constraint ensuring variation_id references a valid product variation. Prevents deletion of variations with existing order_items.';



ALTER TABLE ONLY "public"."product_edit_records"
    ADD CONSTRAINT "product_edit_records_product_id_fkey" FOREIGN KEY ("product_id") REFERENCES "public"."products"("id") ON DELETE CASCADE;



COMMENT ON CONSTRAINT "product_edit_records_product_id_fkey" ON "public"."product_edit_records" IS 'Foreign key constraint ensuring edit records reference valid products. Cascades deletion when product is deleted.';



ALTER TABLE ONLY "public"."product_images"
    ADD CONSTRAINT "product_images_variation_id_fkey" FOREIGN KEY ("variation_id") REFERENCES "public"."product_variations"("id") ON DELETE SET NULL;



COMMENT ON CONSTRAINT "product_images_variation_id_fkey" ON "public"."product_images" IS 'Foreign key constraint linking images to variations. Sets to NULL when variation is deleted, preserving the image.';



ALTER TABLE ONLY "public"."product_order_statistics"
    ADD CONSTRAINT "product_order_statistics_product_id_fkey" FOREIGN KEY ("product_id") REFERENCES "public"."products"("id") ON DELETE CASCADE;



COMMENT ON CONSTRAINT "product_order_statistics_product_id_fkey" ON "public"."product_order_statistics" IS 'Foreign key constraint ensuring statistics reference valid products. Cascades deletion when product is deleted.';



ALTER TABLE ONLY "public"."product_price_rule"
    ADD CONSTRAINT "product_price_rule_variation_id_fkey" FOREIGN KEY ("variation_id") REFERENCES "public"."product_variations"("id") ON DELETE CASCADE;



COMMENT ON CONSTRAINT "product_price_rule_variation_id_fkey" ON "public"."product_price_rule" IS 'Foreign key constraint ensuring price rules reference valid variations. Cascades deletion when variation is deleted.';



ALTER TABLE ONLY "public"."product_ratings"
    ADD CONSTRAINT "product_ratings_order_item_id_fkey" FOREIGN KEY ("order_item_id") REFERENCES "public"."order_items"("id") ON DELETE SET NULL;



COMMENT ON CONSTRAINT "product_ratings_order_item_id_fkey" ON "public"."product_ratings" IS 'Foreign key constraint linking ratings to order items. Sets to NULL when order item is deleted, preserving the review.';



ALTER TABLE ONLY "public"."product_ratings"
    ADD CONSTRAINT "product_ratings_product_id_fkey" FOREIGN KEY ("product_id") REFERENCES "public"."products"("id") ON DELETE CASCADE;



COMMENT ON CONSTRAINT "product_ratings_product_id_fkey" ON "public"."product_ratings" IS 'Foreign key constraint ensuring ratings reference valid products. Cascades deletion when product is deleted.';



ALTER TABLE ONLY "public"."product_variations"
    ADD CONSTRAINT "product_variations_inventory_id_fkey" FOREIGN KEY ("inventory_id") REFERENCES "public"."inventory"("id") ON DELETE RESTRICT;



COMMENT ON CONSTRAINT "product_variations_inventory_id_fkey" ON "public"."product_variations" IS 'Foreign key constraint ensuring variation references valid inventory. Prevents deletion of inventory with active variations.';



ALTER TABLE ONLY "public"."product_variations"
    ADD CONSTRAINT "product_variations_product_id_fkey" FOREIGN KEY ("product_id") REFERENCES "public"."products"("id") ON DELETE CASCADE;



COMMENT ON CONSTRAINT "product_variations_product_id_fkey" ON "public"."product_variations" IS 'Foreign key constraint ensuring variation belongs to a valid product. Cascades deletion when product is deleted.';



ALTER TABLE ONLY "public"."products"
    ADD CONSTRAINT "products_main_pic_id_fkey" FOREIGN KEY ("main_pic_id") REFERENCES "public"."product_images"("id") ON DELETE SET NULL;



COMMENT ON CONSTRAINT "products_main_pic_id_fkey" ON "public"."products" IS 'Foreign key constraint linking product to its main display image. Sets to NULL when image is deleted.';





ALTER PUBLICATION "supabase_realtime" OWNER TO "postgres";


ALTER PUBLICATION "supabase_realtime" ADD TABLE ONLY "public"."order_items";






GRANT USAGE ON SCHEMA "public" TO "postgres";
GRANT USAGE ON SCHEMA "public" TO "anon";
GRANT USAGE ON SCHEMA "public" TO "authenticated";
GRANT USAGE ON SCHEMA "public" TO "service_role";



GRANT ALL ON FUNCTION "public"."gtrgm_in"("cstring") TO "postgres";
GRANT ALL ON FUNCTION "public"."gtrgm_in"("cstring") TO "anon";
GRANT ALL ON FUNCTION "public"."gtrgm_in"("cstring") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gtrgm_in"("cstring") TO "service_role";



GRANT ALL ON FUNCTION "public"."gtrgm_out"("public"."gtrgm") TO "postgres";
GRANT ALL ON FUNCTION "public"."gtrgm_out"("public"."gtrgm") TO "anon";
GRANT ALL ON FUNCTION "public"."gtrgm_out"("public"."gtrgm") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gtrgm_out"("public"."gtrgm") TO "service_role";

























































































































































GRANT ALL ON FUNCTION "public"."gin_extract_query_trgm"("text", "internal", smallint, "internal", "internal", "internal", "internal") TO "postgres";
GRANT ALL ON FUNCTION "public"."gin_extract_query_trgm"("text", "internal", smallint, "internal", "internal", "internal", "internal") TO "anon";
GRANT ALL ON FUNCTION "public"."gin_extract_query_trgm"("text", "internal", smallint, "internal", "internal", "internal", "internal") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gin_extract_query_trgm"("text", "internal", smallint, "internal", "internal", "internal", "internal") TO "service_role";



GRANT ALL ON FUNCTION "public"."gin_extract_value_trgm"("text", "internal") TO "postgres";
GRANT ALL ON FUNCTION "public"."gin_extract_value_trgm"("text", "internal") TO "anon";
GRANT ALL ON FUNCTION "public"."gin_extract_value_trgm"("text", "internal") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gin_extract_value_trgm"("text", "internal") TO "service_role";



GRANT ALL ON FUNCTION "public"."gin_trgm_consistent"("internal", smallint, "text", integer, "internal", "internal", "internal", "internal") TO "postgres";
GRANT ALL ON FUNCTION "public"."gin_trgm_consistent"("internal", smallint, "text", integer, "internal", "internal", "internal", "internal") TO "anon";
GRANT ALL ON FUNCTION "public"."gin_trgm_consistent"("internal", smallint, "text", integer, "internal", "internal", "internal", "internal") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gin_trgm_consistent"("internal", smallint, "text", integer, "internal", "internal", "internal", "internal") TO "service_role";



GRANT ALL ON FUNCTION "public"."gin_trgm_triconsistent"("internal", smallint, "text", integer, "internal", "internal", "internal") TO "postgres";
GRANT ALL ON FUNCTION "public"."gin_trgm_triconsistent"("internal", smallint, "text", integer, "internal", "internal", "internal") TO "anon";
GRANT ALL ON FUNCTION "public"."gin_trgm_triconsistent"("internal", smallint, "text", integer, "internal", "internal", "internal") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gin_trgm_triconsistent"("internal", smallint, "text", integer, "internal", "internal", "internal") TO "service_role";



GRANT ALL ON FUNCTION "public"."gtrgm_compress"("internal") TO "postgres";
GRANT ALL ON FUNCTION "public"."gtrgm_compress"("internal") TO "anon";
GRANT ALL ON FUNCTION "public"."gtrgm_compress"("internal") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gtrgm_compress"("internal") TO "service_role";



GRANT ALL ON FUNCTION "public"."gtrgm_consistent"("internal", "text", smallint, "oid", "internal") TO "postgres";
GRANT ALL ON FUNCTION "public"."gtrgm_consistent"("internal", "text", smallint, "oid", "internal") TO "anon";
GRANT ALL ON FUNCTION "public"."gtrgm_consistent"("internal", "text", smallint, "oid", "internal") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gtrgm_consistent"("internal", "text", smallint, "oid", "internal") TO "service_role";



GRANT ALL ON FUNCTION "public"."gtrgm_decompress"("internal") TO "postgres";
GRANT ALL ON FUNCTION "public"."gtrgm_decompress"("internal") TO "anon";
GRANT ALL ON FUNCTION "public"."gtrgm_decompress"("internal") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gtrgm_decompress"("internal") TO "service_role";



GRANT ALL ON FUNCTION "public"."gtrgm_distance"("internal", "text", smallint, "oid", "internal") TO "postgres";
GRANT ALL ON FUNCTION "public"."gtrgm_distance"("internal", "text", smallint, "oid", "internal") TO "anon";
GRANT ALL ON FUNCTION "public"."gtrgm_distance"("internal", "text", smallint, "oid", "internal") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gtrgm_distance"("internal", "text", smallint, "oid", "internal") TO "service_role";



GRANT ALL ON FUNCTION "public"."gtrgm_options"("internal") TO "postgres";
GRANT ALL ON FUNCTION "public"."gtrgm_options"("internal") TO "anon";
GRANT ALL ON FUNCTION "public"."gtrgm_options"("internal") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gtrgm_options"("internal") TO "service_role";



GRANT ALL ON FUNCTION "public"."gtrgm_penalty"("internal", "internal", "internal") TO "postgres";
GRANT ALL ON FUNCTION "public"."gtrgm_penalty"("internal", "internal", "internal") TO "anon";
GRANT ALL ON FUNCTION "public"."gtrgm_penalty"("internal", "internal", "internal") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gtrgm_penalty"("internal", "internal", "internal") TO "service_role";



GRANT ALL ON FUNCTION "public"."gtrgm_picksplit"("internal", "internal") TO "postgres";
GRANT ALL ON FUNCTION "public"."gtrgm_picksplit"("internal", "internal") TO "anon";
GRANT ALL ON FUNCTION "public"."gtrgm_picksplit"("internal", "internal") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gtrgm_picksplit"("internal", "internal") TO "service_role";



GRANT ALL ON FUNCTION "public"."gtrgm_same"("public"."gtrgm", "public"."gtrgm", "internal") TO "postgres";
GRANT ALL ON FUNCTION "public"."gtrgm_same"("public"."gtrgm", "public"."gtrgm", "internal") TO "anon";
GRANT ALL ON FUNCTION "public"."gtrgm_same"("public"."gtrgm", "public"."gtrgm", "internal") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gtrgm_same"("public"."gtrgm", "public"."gtrgm", "internal") TO "service_role";



GRANT ALL ON FUNCTION "public"."gtrgm_union"("internal", "internal") TO "postgres";
GRANT ALL ON FUNCTION "public"."gtrgm_union"("internal", "internal") TO "anon";
GRANT ALL ON FUNCTION "public"."gtrgm_union"("internal", "internal") TO "authenticated";
GRANT ALL ON FUNCTION "public"."gtrgm_union"("internal", "internal") TO "service_role";



GRANT ALL ON TABLE "public"."uk_addresses" TO "anon";
GRANT ALL ON TABLE "public"."uk_addresses" TO "authenticated";
GRANT ALL ON TABLE "public"."uk_addresses" TO "service_role";



GRANT ALL ON FUNCTION "public"."search_uk_addresses_by_postcode"("p_input" "text", "p_limit" integer) TO "anon";
GRANT ALL ON FUNCTION "public"."search_uk_addresses_by_postcode"("p_input" "text", "p_limit" integer) TO "authenticated";
GRANT ALL ON FUNCTION "public"."search_uk_addresses_by_postcode"("p_input" "text", "p_limit" integer) TO "service_role";



GRANT ALL ON FUNCTION "public"."set_limit"(real) TO "postgres";
GRANT ALL ON FUNCTION "public"."set_limit"(real) TO "anon";
GRANT ALL ON FUNCTION "public"."set_limit"(real) TO "authenticated";
GRANT ALL ON FUNCTION "public"."set_limit"(real) TO "service_role";



GRANT ALL ON FUNCTION "public"."show_limit"() TO "postgres";
GRANT ALL ON FUNCTION "public"."show_limit"() TO "anon";
GRANT ALL ON FUNCTION "public"."show_limit"() TO "authenticated";
GRANT ALL ON FUNCTION "public"."show_limit"() TO "service_role";



GRANT ALL ON FUNCTION "public"."show_trgm"("text") TO "postgres";
GRANT ALL ON FUNCTION "public"."show_trgm"("text") TO "anon";
GRANT ALL ON FUNCTION "public"."show_trgm"("text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."show_trgm"("text") TO "service_role";



GRANT ALL ON FUNCTION "public"."similarity"("text", "text") TO "postgres";
GRANT ALL ON FUNCTION "public"."similarity"("text", "text") TO "anon";
GRANT ALL ON FUNCTION "public"."similarity"("text", "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."similarity"("text", "text") TO "service_role";



GRANT ALL ON FUNCTION "public"."similarity_dist"("text", "text") TO "postgres";
GRANT ALL ON FUNCTION "public"."similarity_dist"("text", "text") TO "anon";
GRANT ALL ON FUNCTION "public"."similarity_dist"("text", "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."similarity_dist"("text", "text") TO "service_role";



GRANT ALL ON FUNCTION "public"."similarity_op"("text", "text") TO "postgres";
GRANT ALL ON FUNCTION "public"."similarity_op"("text", "text") TO "anon";
GRANT ALL ON FUNCTION "public"."similarity_op"("text", "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."similarity_op"("text", "text") TO "service_role";



GRANT ALL ON FUNCTION "public"."strict_word_similarity"("text", "text") TO "postgres";
GRANT ALL ON FUNCTION "public"."strict_word_similarity"("text", "text") TO "anon";
GRANT ALL ON FUNCTION "public"."strict_word_similarity"("text", "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."strict_word_similarity"("text", "text") TO "service_role";



GRANT ALL ON FUNCTION "public"."strict_word_similarity_commutator_op"("text", "text") TO "postgres";
GRANT ALL ON FUNCTION "public"."strict_word_similarity_commutator_op"("text", "text") TO "anon";
GRANT ALL ON FUNCTION "public"."strict_word_similarity_commutator_op"("text", "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."strict_word_similarity_commutator_op"("text", "text") TO "service_role";



GRANT ALL ON FUNCTION "public"."strict_word_similarity_dist_commutator_op"("text", "text") TO "postgres";
GRANT ALL ON FUNCTION "public"."strict_word_similarity_dist_commutator_op"("text", "text") TO "anon";
GRANT ALL ON FUNCTION "public"."strict_word_similarity_dist_commutator_op"("text", "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."strict_word_similarity_dist_commutator_op"("text", "text") TO "service_role";



GRANT ALL ON FUNCTION "public"."strict_word_similarity_dist_op"("text", "text") TO "postgres";
GRANT ALL ON FUNCTION "public"."strict_word_similarity_dist_op"("text", "text") TO "anon";
GRANT ALL ON FUNCTION "public"."strict_word_similarity_dist_op"("text", "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."strict_word_similarity_dist_op"("text", "text") TO "service_role";



GRANT ALL ON FUNCTION "public"."strict_word_similarity_op"("text", "text") TO "postgres";
GRANT ALL ON FUNCTION "public"."strict_word_similarity_op"("text", "text") TO "anon";
GRANT ALL ON FUNCTION "public"."strict_word_similarity_op"("text", "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."strict_word_similarity_op"("text", "text") TO "service_role";



GRANT ALL ON FUNCTION "public"."word_similarity"("text", "text") TO "postgres";
GRANT ALL ON FUNCTION "public"."word_similarity"("text", "text") TO "anon";
GRANT ALL ON FUNCTION "public"."word_similarity"("text", "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."word_similarity"("text", "text") TO "service_role";



GRANT ALL ON FUNCTION "public"."word_similarity_commutator_op"("text", "text") TO "postgres";
GRANT ALL ON FUNCTION "public"."word_similarity_commutator_op"("text", "text") TO "anon";
GRANT ALL ON FUNCTION "public"."word_similarity_commutator_op"("text", "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."word_similarity_commutator_op"("text", "text") TO "service_role";



GRANT ALL ON FUNCTION "public"."word_similarity_dist_commutator_op"("text", "text") TO "postgres";
GRANT ALL ON FUNCTION "public"."word_similarity_dist_commutator_op"("text", "text") TO "anon";
GRANT ALL ON FUNCTION "public"."word_similarity_dist_commutator_op"("text", "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."word_similarity_dist_commutator_op"("text", "text") TO "service_role";



GRANT ALL ON FUNCTION "public"."word_similarity_dist_op"("text", "text") TO "postgres";
GRANT ALL ON FUNCTION "public"."word_similarity_dist_op"("text", "text") TO "anon";
GRANT ALL ON FUNCTION "public"."word_similarity_dist_op"("text", "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."word_similarity_dist_op"("text", "text") TO "service_role";



GRANT ALL ON FUNCTION "public"."word_similarity_op"("text", "text") TO "postgres";
GRANT ALL ON FUNCTION "public"."word_similarity_op"("text", "text") TO "anon";
GRANT ALL ON FUNCTION "public"."word_similarity_op"("text", "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."word_similarity_op"("text", "text") TO "service_role";


















GRANT ALL ON TABLE "public"."categories" TO "anon";
GRANT ALL ON TABLE "public"."categories" TO "authenticated";
GRANT ALL ON TABLE "public"."categories" TO "service_role";



GRANT ALL ON SEQUENCE "public"."categories_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."categories_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."categories_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."category_products" TO "anon";
GRANT ALL ON TABLE "public"."category_products" TO "authenticated";
GRANT ALL ON TABLE "public"."category_products" TO "service_role";



GRANT ALL ON SEQUENCE "public"."category_products_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."category_products_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."category_products_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."customer_info" TO "anon";
GRANT ALL ON TABLE "public"."customer_info" TO "authenticated";
GRANT ALL ON TABLE "public"."customer_info" TO "service_role";



GRANT ALL ON SEQUENCE "public"."customer_info_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."customer_info_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."customer_info_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."discount_codes" TO "anon";
GRANT ALL ON TABLE "public"."discount_codes" TO "authenticated";
GRANT ALL ON TABLE "public"."discount_codes" TO "service_role";



GRANT ALL ON SEQUENCE "public"."discount_codes_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."discount_codes_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."discount_codes_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."edm_activity_emails" TO "anon";
GRANT ALL ON TABLE "public"."edm_activity_emails" TO "authenticated";
GRANT ALL ON TABLE "public"."edm_activity_emails" TO "service_role";



GRANT ALL ON SEQUENCE "public"."edm_activity_emails_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."edm_activity_emails_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."edm_activity_emails_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."edm_activity_recipient" TO "anon";
GRANT ALL ON TABLE "public"."edm_activity_recipient" TO "authenticated";
GRANT ALL ON TABLE "public"."edm_activity_recipient" TO "service_role";



GRANT ALL ON SEQUENCE "public"."edm_activity_recipient_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."edm_activity_recipient_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."edm_activity_recipient_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."email_list" TO "anon";
GRANT ALL ON TABLE "public"."email_list" TO "authenticated";
GRANT ALL ON TABLE "public"."email_list" TO "service_role";



GRANT ALL ON SEQUENCE "public"."email_list_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."email_list_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."email_list_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."gcs_list" TO "anon";
GRANT ALL ON TABLE "public"."gcs_list" TO "authenticated";
GRANT ALL ON TABLE "public"."gcs_list" TO "service_role";



GRANT ALL ON SEQUENCE "public"."gcs_list_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."gcs_list_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."gcs_list_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."google_product_taxonomy" TO "anon";
GRANT ALL ON TABLE "public"."google_product_taxonomy" TO "authenticated";
GRANT ALL ON TABLE "public"."google_product_taxonomy" TO "service_role";



GRANT ALL ON TABLE "public"."inventory" TO "anon";
GRANT ALL ON TABLE "public"."inventory" TO "authenticated";
GRANT ALL ON TABLE "public"."inventory" TO "service_role";



GRANT ALL ON SEQUENCE "public"."inventory_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."inventory_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."inventory_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."order_items" TO "anon";
GRANT ALL ON TABLE "public"."order_items" TO "authenticated";
GRANT ALL ON TABLE "public"."order_items" TO "service_role";



GRANT ALL ON SEQUENCE "public"."order_items_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."order_items_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."order_items_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."orders" TO "anon";
GRANT ALL ON TABLE "public"."orders" TO "authenticated";
GRANT ALL ON TABLE "public"."orders" TO "service_role";



GRANT ALL ON SEQUENCE "public"."orders_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."orders_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."orders_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."product_edit_records" TO "anon";
GRANT ALL ON TABLE "public"."product_edit_records" TO "authenticated";
GRANT ALL ON TABLE "public"."product_edit_records" TO "service_role";



GRANT ALL ON SEQUENCE "public"."product_edit_records_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."product_edit_records_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."product_edit_records_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."product_images" TO "anon";
GRANT ALL ON TABLE "public"."product_images" TO "authenticated";
GRANT ALL ON TABLE "public"."product_images" TO "service_role";



GRANT ALL ON SEQUENCE "public"."product_images_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."product_images_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."product_images_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."product_order_statistics" TO "anon";
GRANT ALL ON TABLE "public"."product_order_statistics" TO "authenticated";
GRANT ALL ON TABLE "public"."product_order_statistics" TO "service_role";



GRANT ALL ON SEQUENCE "public"."product_order_statistics_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."product_order_statistics_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."product_order_statistics_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."product_price_rule" TO "anon";
GRANT ALL ON TABLE "public"."product_price_rule" TO "authenticated";
GRANT ALL ON TABLE "public"."product_price_rule" TO "service_role";



GRANT ALL ON SEQUENCE "public"."product_price_rule_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."product_price_rule_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."product_price_rule_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."product_ratings" TO "anon";
GRANT ALL ON TABLE "public"."product_ratings" TO "authenticated";
GRANT ALL ON TABLE "public"."product_ratings" TO "service_role";



GRANT ALL ON SEQUENCE "public"."product_ratings_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."product_ratings_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."product_ratings_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."product_variations" TO "anon";
GRANT ALL ON TABLE "public"."product_variations" TO "authenticated";
GRANT ALL ON TABLE "public"."product_variations" TO "service_role";



GRANT ALL ON SEQUENCE "public"."product_variations_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."product_variations_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."product_variations_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."products" TO "anon";
GRANT ALL ON TABLE "public"."products" TO "authenticated";
GRANT ALL ON TABLE "public"."products" TO "service_role";



GRANT ALL ON SEQUENCE "public"."products_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."products_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."products_id_seq" TO "service_role";



GRANT ALL ON SEQUENCE "public"."uk_addresses_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."uk_addresses_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."uk_addresses_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."users" TO "anon";
GRANT ALL ON TABLE "public"."users" TO "authenticated";
GRANT ALL ON TABLE "public"."users" TO "service_role";



GRANT ALL ON SEQUENCE "public"."users_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."users_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."users_id_seq" TO "service_role";









ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON SEQUENCES TO "postgres";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON SEQUENCES TO "anon";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON SEQUENCES TO "authenticated";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON SEQUENCES TO "service_role";






ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON FUNCTIONS TO "postgres";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON FUNCTIONS TO "anon";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON FUNCTIONS TO "authenticated";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON FUNCTIONS TO "service_role";






ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON TABLES TO "postgres";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON TABLES TO "anon";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON TABLES TO "authenticated";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON TABLES TO "service_role";































