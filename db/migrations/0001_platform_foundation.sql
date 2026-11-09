BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TYPE membership_role AS ENUM (
    'business_owner', 'property_owner', 'broker_agent', 'property_manager', 'staff'
);
CREATE TYPE membership_status AS ENUM ('invited', 'active', 'suspended');
CREATE TYPE property_status AS ENUM ('draft', 'pending_review', 'published', 'rejected', 'inactive');
CREATE TYPE booking_status AS ENUM (
    'pending', 'confirmed', 'checked_in', 'completed', 'cancelled', 'rejected', 'expired'
);
CREATE TYPE reservation_status AS ENUM ('active', 'released');
CREATE TYPE reservation_kind AS ENUM ('booking', 'owner_block', 'maintenance');
CREATE TYPE payment_status AS ENUM ('pending', 'processing', 'succeeded', 'failed', 'partially_refunded', 'refunded');
CREATE TYPE payment_method AS ENUM ('mpesa', 'card', 'bank_transfer', 'cash', 'manual');
CREATE TYPE commission_status AS ENUM ('pending', 'approved', 'paid', 'void');

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    phone text,
    password_hash text NOT NULL,
    full_name text NOT NULL,
    is_platform_admin boolean NOT NULL DEFAULT false,
    email_verified_at timestamptz,
    phone_verified_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT users_email_lowercase CHECK (email = lower(email))
);
CREATE UNIQUE INDEX users_email_unique ON users (email) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX users_phone_unique ON users (phone) WHERE phone IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE businesses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    legal_name text,
    default_currency char(3) NOT NULL DEFAULT 'KES',
    timezone text NOT NULL DEFAULT 'Africa/Nairobi',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT businesses_currency_uppercase CHECK (default_currency = upper(default_currency))
);

CREATE TABLE business_memberships (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    role membership_role NOT NULL,
    status membership_status NOT NULL DEFAULT 'invited',
    invited_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (business_id, user_id)
);
CREATE INDEX business_memberships_user_status_idx ON business_memberships (user_id, status);

CREATE TABLE properties (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    owner_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    manager_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    name text NOT NULL,
    slug text NOT NULL,
    description text NOT NULL DEFAULT '',
    category text NOT NULL,
    country_code char(2) NOT NULL DEFAULT 'KE',
    county text,
    city text NOT NULL,
    address text,
    latitude numeric(9,6),
    longitude numeric(9,6),
    status property_status NOT NULL DEFAULT 'draft',
    approved_by uuid REFERENCES users(id) ON DELETE SET NULL,
    approved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (business_id, slug),
    UNIQUE (id, business_id),
    CONSTRAINT properties_latitude_range CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
    CONSTRAINT properties_longitude_range CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180)
);
CREATE INDEX properties_discovery_idx ON properties (country_code, city, status) WHERE deleted_at IS NULL;
CREATE INDEX properties_business_status_idx ON properties (business_id, status) WHERE deleted_at IS NULL;
CREATE INDEX properties_owner_idx ON properties (owner_user_id) WHERE owner_user_id IS NOT NULL;

CREATE TABLE property_units (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id uuid NOT NULL REFERENCES properties(id) ON DELETE RESTRICT,
    name text NOT NULL,
    unit_type text NOT NULL DEFAULT 'entire_property',
    capacity integer NOT NULL CHECK (capacity > 0),
    bedrooms integer NOT NULL DEFAULT 0 CHECK (bedrooms >= 0),
    bathrooms numeric(4,1) NOT NULL DEFAULT 1 CHECK (bathrooms >= 0),
    base_nightly_rate numeric(12,2) NOT NULL CHECK (base_nightly_rate >= 0),
    weekend_nightly_rate numeric(12,2) CHECK (weekend_nightly_rate IS NULL OR weekend_nightly_rate >= 0),
    currency char(3) NOT NULL DEFAULT 'KES',
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (property_id, name),
    UNIQUE (id, property_id),
    CONSTRAINT property_units_currency_uppercase CHECK (currency = upper(currency))
);
CREATE INDEX property_units_active_capacity_idx ON property_units (capacity, property_id) WHERE is_active;

CREATE TABLE property_media (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id uuid NOT NULL REFERENCES properties(id) ON DELETE RESTRICT,
    media_type text NOT NULL CHECK (media_type IN ('image', 'video')),
    storage_key text NOT NULL,
    public_url text NOT NULL,
    alt_text text NOT NULL DEFAULT '',
    position integer NOT NULL DEFAULT 0 CHECK (position >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (property_id, storage_key)
);
CREATE INDEX property_media_order_idx ON property_media (property_id, position);

CREATE TABLE amenities (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug text NOT NULL UNIQUE,
    name text NOT NULL UNIQUE
);

CREATE TABLE property_amenities (
    property_id uuid NOT NULL REFERENCES properties(id) ON DELETE RESTRICT,
    amenity_id uuid NOT NULL REFERENCES amenities(id) ON DELETE RESTRICT,
    PRIMARY KEY (property_id, amenity_id)
);

CREATE TABLE property_rates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    unit_id uuid NOT NULL REFERENCES property_units(id) ON DELETE RESTRICT,
    starts_on date NOT NULL,
    ends_on date NOT NULL,
    nightly_rate numeric(12,2) NOT NULL CHECK (nightly_rate >= 0),
    minimum_nights integer NOT NULL DEFAULT 1 CHECK (minimum_nights > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (starts_on < ends_on),
    EXCLUDE USING gist (unit_id WITH =, daterange(starts_on, ends_on, '[)') WITH &&)
);
CREATE INDEX property_rates_unit_dates_idx ON property_rates (unit_id, starts_on, ends_on);

CREATE TABLE customers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    full_name text NOT NULL,
    email text,
    phone text,
    preferences jsonb NOT NULL DEFAULT '{}'::jsonb,
    internal_notes text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (id, business_id)
);
CREATE INDEX customers_business_email_idx ON customers (business_id, lower(email)) WHERE deleted_at IS NULL;
CREATE INDEX customers_business_phone_idx ON customers (business_id, phone) WHERE phone IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE bookings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    property_id uuid NOT NULL,
    unit_id uuid NOT NULL,
    customer_id uuid,
    customer_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    reference text NOT NULL UNIQUE,
    guest_name text NOT NULL,
    guest_email text NOT NULL,
    guest_phone text,
    check_in date NOT NULL,
    check_out date NOT NULL,
    guest_count integer NOT NULL CHECK (guest_count > 0),
    status booking_status NOT NULL DEFAULT 'pending',
    currency char(3) NOT NULL,
    subtotal numeric(12,2) NOT NULL CHECK (subtotal >= 0),
    taxes numeric(12,2) NOT NULL DEFAULT 0 CHECK (taxes >= 0),
    total_amount numeric(12,2) NOT NULL CHECK (total_amount >= 0),
    amount_paid numeric(12,2) NOT NULL DEFAULT 0 CHECK (amount_paid >= 0),
    expires_at timestamptz,
    confirmed_at timestamptz,
    cancelled_at timestamptz,
    cancellation_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (check_in < check_out),
    CHECK (guest_count <= 100),
    CHECK (amount_paid <= total_amount),
    CHECK (total_amount = subtotal + taxes),
    CHECK (currency = upper(currency)),
    FOREIGN KEY (property_id, business_id) REFERENCES properties(id, business_id) ON DELETE RESTRICT,
    FOREIGN KEY (unit_id, property_id) REFERENCES property_units(id, property_id) ON DELETE RESTRICT,
    FOREIGN KEY (customer_id, business_id) REFERENCES customers(id, business_id) ON DELETE SET NULL (customer_id),
    UNIQUE (id, business_id, currency),
    UNIQUE (id, unit_id)
);
CREATE INDEX bookings_business_created_idx ON bookings (business_id, created_at DESC);
CREATE INDEX bookings_business_status_dates_idx ON bookings (business_id, status, check_in, check_out);
CREATE INDEX bookings_customer_idx ON bookings (customer_id, created_at DESC) WHERE customer_id IS NOT NULL;
CREATE INDEX bookings_unit_dates_idx ON bookings (unit_id, check_in, check_out);

CREATE TABLE unit_reservations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    unit_id uuid NOT NULL REFERENCES property_units(id) ON DELETE RESTRICT,
    booking_id uuid UNIQUE,
    kind reservation_kind NOT NULL,
    status reservation_status NOT NULL DEFAULT 'active',
    reserved_dates daterange NOT NULL,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    expires_at timestamptz,
    released_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (NOT isempty(reserved_dates)),
    CHECK ((kind = 'booking' AND booking_id IS NOT NULL) OR (kind <> 'booking' AND booking_id IS NULL)),
    FOREIGN KEY (booking_id, unit_id) REFERENCES bookings(id, unit_id) ON DELETE RESTRICT,
    EXCLUDE USING gist (unit_id WITH =, reserved_dates WITH &&) WHERE (status = 'active')
);
CREATE INDEX unit_reservations_expiry_idx ON unit_reservations (expires_at) WHERE status = 'active' AND expires_at IS NOT NULL;

CREATE TABLE payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    booking_id uuid NOT NULL,
    method payment_method NOT NULL,
    status payment_status NOT NULL DEFAULT 'pending',
    amount numeric(12,2) NOT NULL CHECK (amount > 0),
    currency char(3) NOT NULL,
    provider text NOT NULL,
    provider_reference text,
    idempotency_key text NOT NULL,
    verified_at timestamptz,
    failure_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, idempotency_key),
    FOREIGN KEY (booking_id, business_id, currency) REFERENCES bookings(id, business_id, currency) ON DELETE RESTRICT,
    CHECK (currency = upper(currency))
);
CREATE UNIQUE INDEX payments_provider_reference_unique ON payments (provider, provider_reference) WHERE provider_reference IS NOT NULL;
CREATE INDEX payments_business_created_idx ON payments (business_id, created_at DESC);
CREATE INDEX payments_booking_status_idx ON payments (booking_id, status);

CREATE TABLE commissions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    booking_id uuid NOT NULL,
    agent_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    owner_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    commission_type text NOT NULL CHECK (commission_type IN ('percentage', 'fixed')),
    rate numeric(8,4) NOT NULL CHECK (rate >= 0),
    amount numeric(12,2) NOT NULL CHECK (amount >= 0),
    currency char(3) NOT NULL,
    status commission_status NOT NULL DEFAULT 'pending',
    created_at timestamptz NOT NULL DEFAULT now(),
    paid_at timestamptz,
    FOREIGN KEY (booking_id, business_id, currency) REFERENCES bookings(id, business_id, currency) ON DELETE RESTRICT,
    CHECK (currency = upper(currency))
);
CREATE INDEX commissions_business_status_idx ON commissions (business_id, status, created_at DESC);
CREATE INDEX commissions_agent_status_idx ON commissions (agent_user_id, status) WHERE agent_user_id IS NOT NULL;
CREATE UNIQUE INDEX commissions_agent_booking_unique ON commissions (booking_id, agent_user_id) WHERE agent_user_id IS NOT NULL;
CREATE UNIQUE INDEX commissions_unassigned_booking_unique ON commissions (booking_id) WHERE agent_user_id IS NULL;

CREATE TABLE audit_logs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    business_id uuid REFERENCES businesses(id) ON DELETE RESTRICT,
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    request_id text,
    ip_address inet,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_business_created_idx ON audit_logs (business_id, created_at DESC);
CREATE INDEX audit_logs_target_idx ON audit_logs (target_type, target_id, created_at DESC);

COMMIT;