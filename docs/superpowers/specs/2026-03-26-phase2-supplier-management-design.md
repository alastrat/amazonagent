# Phase 2: Supplier Management + Outreach — Design Spec

**Date:** 2026-03-26
**Status:** Draft — pending review
**Scope:** Supplier CRM, outreach pipeline, quote tracking, tenant automation settings, channel model

---

## 1. Goal

Extend the deal lifecycle past "approved" into active supplier management. When a user approves a deal, the system surfaces supplier candidates (from the research pipeline or manual entry), generates outreach emails, sends them, tracks responses, and lets the user select a winning supplier.

Human-in-the-loop by default. Full automation available as a tenant setting.

---

## 2. Key Design Decisions

### Channel as a first-class concept
A deal is a product opportunity, not an Amazon-specific entity. The sales channel (Amazon FBA, Shopify, FB Marketplace, eBay) is a dimension that can be experimented on via autoresearch. Phase 2 adds the `Channel` model but only implements Amazon FBA. Multi-channel listing comes in Phase 4.

### Supplier types
Three supplier types to future-proof for dropshipping validation:
- **Wholesale** — primary, ships to FBA (Phase 2 focus)
- **Validation** — dropship platforms for off-Amazon demand testing (future)
- **Hybrid** — US distributors who can both dropship and sell wholesale (future)

Phase 2 implements wholesale only. The `type` field exists in the model for forward compatibility.

### Supplier sources
Suppliers enter the system from three sources:
- **Pipeline** — discovered by the Supplier Agent during research
- **Manual** — user adds suppliers they already have relationships with
- **API import** — third-party integrations like SellerAssistant (future, behind interface)

### In-app email sending
Outreach emails are sent from the platform via an email provider (SendGrid/SES). Full tracking: sent, delivered, opened, responded. Email provider is an adapter behind an interface.

### Automation levels
Per-tenant settings control how much human involvement is required:

| Setting | Manual (default) | Assisted | Auto |
|---------|:---:|:---:|:---:|
| On deal approval → sourcing | User clicks "Start Sourcing" | Auto-advance | Auto-advance |
| Outreach sending | Approve each message | Approve first, template rest | Auto-send to all |
| Follow-up scheduling | Manual | Suggest after N days | Auto-send after N days |
| Supplier selection | Manual pick | Recommend best, user confirms | Auto-select lowest-price authorized |

Default: everything requires human approval. Users can escalate automation per-setting.

---

## 3. Domain Models

### Channel (new)

```
Channel {
  id, tenant_id,
  type: amazon_fba | shopify | fb_marketplace | ebay | custom,
  name,
  credentials (JSON, encrypted),
  fulfillment_strategy: fba | fbm | dropship | self_fulfill,
  enabled,
  created_at, updated_at
}
```

Phase 2: only `amazon_fba` implemented. The model exists for forward compatibility.

### Supplier (extended from stub)

```
Supplier {
  id, tenant_id,
  type: wholesale | validation | hybrid,
  name, website,
  authorization_status: pending | verified | rejected,
  reliability_score (float, nullable),
  source: pipeline | manual | api_import,
  created_at, updated_at
}
```

### SupplierContact (new)

```
SupplierContact {
  id, supplier_id, tenant_id,
  name, email, phone, role,
  is_primary (bool),
  created_at
}
```

### SupplierNote (new)

```
SupplierNote {
  id, supplier_id, tenant_id,
  content (text),
  author_id,
  created_at
}
```

### SupplierQuote (new)

```
SupplierQuote {
  id, supplier_id, deal_id, tenant_id,
  unit_price, moq, lead_time_days,
  shipping_terms, valid_until,
  status: pending | accepted | rejected | expired,
  created_at, updated_at
}
```

### Outreach (new)

```
Outreach {
  id, deal_id, supplier_id, tenant_id,
  contact_id (which supplier contact),
  subject, body,
  status: draft | approved | queued | sent | delivered | opened | responded | failed,
  approved_by, approved_at,
  sent_at, delivered_at, opened_at, responded_at,
  follow_up_of (nullable — links to parent outreach for follow-ups),
  follow_up_scheduled_at,
  response_summary (text — AI summary of supplier reply, nullable),
  created_at, updated_at
}
```

### TenantSettings (new)

```
TenantSettings {
  id, tenant_id,
  auto_advance_on_approve: bool (default true),
  outreach_auto_send: never | first_approved | always (default never),
  follow_up_enabled: bool (default false),
  follow_up_days: int (default 7),
  supplier_auto_select: bool (default false),
  email_provider: sendgrid | ses (default sendgrid),
  email_from_name,
  email_from_address,
  created_at, updated_at
}
```

### Deal (extended)

Add to existing Deal model:
```
  supplier_id (nullable — set when supplier is selected)
  channel_id (nullable — set during sourcing or listing)
```

`supplier_id` already exists in the model. `channel_id` is new.

---

## 4. The Sourcing Flow

```
Deal "approved"
  │
  ├── If auto_advance_on_approve: auto-transition
  ├── Else: user clicks "Start Sourcing"
  ▼
Deal → "sourcing"
  │
  ▼
System creates suppliers from pipeline's SupplierCandidates
  ├── Each SupplierCandidate → Supplier (if not already exists, match by name+website)
  ├── Creates SupplierQuote per candidate
  ├── Creates Outreach draft per candidate (from pipeline's outreach_drafts)
  │
  ▼
Outreach drafts ready for review
  │
  ├── If outreach_auto_send == "always": send immediately
  ├── If outreach_auto_send == "first_approved": user approves first, rest use same template
  ├── If outreach_auto_send == "never": user reviews/edits each draft → approves → sends
  │
  ▼
Emails sent via EmailProvider adapter
  │
  ▼
Track delivery + opens (via email provider webhooks)
  │
  ├── If follow_up_enabled: auto-schedule follow-up after follow_up_days
  ├── Else: user manually follows up
  │
  ▼
Supplier responds → user logs response / system detects via email tracking
  │
  ▼
User creates SupplierQuote from response (or system suggests via AI parsing)
  │
  ▼
User reviews all quotes for deal → selects winner
  │
  ▼
POST /deals/:id/select-supplier
  ├── Deal.supplier_id = selected supplier
  ├── Deal → "procuring" (Phase 3)
  ├── Domain event: deal_supplier_selected
```

---

## 5. Interfaces (Ports)

### EmailProvider (new port)

```go
type EmailProvider interface {
    Send(ctx context.Context, email Email) (*EmailResult, error)
}

type Email struct {
    From    EmailAddress
    To      EmailAddress
    Subject string
    Body    string // HTML
}

type EmailResult struct {
    MessageID  string
    Status     string // "sent", "queued"
}
```

Adapters: SendGrid, SES (Phase 2 implements SendGrid).

### SupplierMatcher (new port — future)

```go
type SupplierMatcher interface {
    FindSuppliers(ctx context.Context, product ProductInfo) ([]SupplierMatch, error)
}
```

For third-party integrations (SellerAssistant, etc.). Stub in Phase 2.

---

## 6. New Repositories

```
SupplierRepo {
    Create, GetByID, List(tenantID, filter), Update
    FindByNameAndWebsite(tenantID, name, website) — for deduplication
}

SupplierContactRepo {
    Create, List(supplierID), Delete
}

SupplierNoteRepo {
    Create, List(supplierID)
}

SupplierQuoteRepo {
    Create, GetByID, List(dealID), Update
    Accept(quoteID) — sets status + rejects others for same deal
}

OutreachRepo {
    Create, GetByID, List(dealID), Update
    ListPendingFollowUps(tenantID) — for follow-up scheduler
}

TenantSettingsRepo {
    Get(tenantID), Upsert
}

ChannelRepo {
    Create, GetByID, List(tenantID), Update
}
```

---

## 7. New Services

### SupplierService
- CRUD for suppliers, contacts, notes
- `CreateFromPipelineCandidate(candidate SupplierCandidate)` — deduplicates by name+website
- `ListForDeal(dealID)` — suppliers linked via quotes to a deal

### OutreachService
- Create drafts from pipeline data
- Approve, send (calls EmailProvider), track status
- Schedule follow-ups
- `InitiateSourcing(dealID)` — creates suppliers + quotes + outreach drafts from deal's pipeline data

### QuoteService
- CRUD for quotes
- `AcceptQuote(quoteID)` — marks as accepted, rejects others, sets deal.supplier_id

### SourcingService (orchestrator)
- `StartSourcing(dealID)` — transitions deal, calls OutreachService.InitiateSourcing
- `SelectSupplier(dealID, supplierID)` — accepts best quote, transitions deal to procuring
- Respects TenantSettings for automation levels

### TenantSettingsService
- Get/update settings
- `EnsureDefaults(tenantID)` — creates default settings if none exist

---

## 8. API Endpoints

```
# Suppliers
GET    /suppliers                        List tenant's suppliers (with filter by type, source)
POST   /suppliers                        Create supplier manually
GET    /suppliers/:id                    Detail + contacts + notes + quotes
PUT    /suppliers/:id                    Update supplier
POST   /suppliers/:id/contacts           Add contact
POST   /suppliers/:id/notes              Add note

# Deal sourcing
POST   /deals/:id/start-sourcing         Start sourcing (manual trigger)
GET    /deals/:id/suppliers              Supplier candidates for deal (with quotes)
POST   /deals/:id/select-supplier        Select supplier → move to procuring
  body: { supplier_id, quote_id }

# Outreach
GET    /deals/:id/outreach               List outreach for deal
POST   /deals/:id/outreach               Create new outreach draft
POST   /outreach/:id/approve             Approve for sending
POST   /outreach/:id/send                Send email
POST   /outreach/:id/follow-up           Create follow-up draft
PUT    /outreach/:id                     Edit draft

# Quotes
GET    /deals/:id/quotes                 List quotes for deal
POST   /deals/:id/quotes                 Add quote manually
POST   /quotes/:id/accept               Accept quote

# Channels
GET    /channels                         List channels
POST   /channels                         Create channel
PUT    /channels/:id                     Update channel

# Settings
GET    /settings                         Get tenant settings
PUT    /settings                         Update tenant settings
```

---

## 9. Frontend Pages

### Supplier list page (`/suppliers`)
- Table: name, type, authorization, source, deal count, reliability score
- Filters: type, authorization status, source
- "Add Supplier" button

### Supplier detail page (`/suppliers/:id`)
- Info card: name, website, type, authorization
- Contacts section: list + add
- Notes section: timeline + add
- Related deals: table of deals linked via quotes
- Outreach history: all outreach sent to this supplier

### Deal detail — Sourcing tab (extension of existing deal detail page)
- Visible when deal status is "sourcing"
- Supplier candidates table: name, price, MOQ, lead time, authorization, outreach status
- Outreach section per supplier: draft preview, edit, approve, send buttons
- Quotes section: add/view quotes, accept button
- "Select Supplier" button → confirms and advances deal

### Settings page (extension of existing)
- Automation section: toggle each setting
- Email configuration: provider, from name, from address
- Channel management section (Phase 2: just shows Amazon FBA as default)

---

## 10. Database Migration

```sql
-- Channels
CREATE TABLE channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    type TEXT NOT NULL DEFAULT 'amazon_fba',
    name TEXT NOT NULL,
    credentials JSONB NOT NULL DEFAULT '{}',
    fulfillment_strategy TEXT NOT NULL DEFAULT 'fba',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Extend suppliers
ALTER TABLE ... (or recreate since stub was minimal)
-- Full supplier table with type, source, contacts, notes

CREATE TABLE supplier_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    supplier_id UUID NOT NULL REFERENCES suppliers(id),
    tenant_id UUID NOT NULL,
    name TEXT NOT NULL,
    email TEXT,
    phone TEXT,
    role TEXT,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE supplier_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    supplier_id UUID NOT NULL REFERENCES suppliers(id),
    tenant_id UUID NOT NULL,
    content TEXT NOT NULL,
    author_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE supplier_quotes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    supplier_id UUID NOT NULL REFERENCES suppliers(id),
    deal_id UUID NOT NULL REFERENCES deals(id),
    tenant_id UUID NOT NULL,
    unit_price NUMERIC(10,2) NOT NULL,
    moq INT NOT NULL DEFAULT 1,
    lead_time_days INT,
    shipping_terms TEXT,
    valid_until TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE outreach (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deal_id UUID NOT NULL REFERENCES deals(id),
    supplier_id UUID NOT NULL REFERENCES suppliers(id),
    tenant_id UUID NOT NULL,
    contact_id UUID REFERENCES supplier_contacts(id),
    subject TEXT NOT NULL,
    body TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    approved_by TEXT,
    approved_at TIMESTAMPTZ,
    sent_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    opened_at TIMESTAMPTZ,
    responded_at TIMESTAMPTZ,
    follow_up_of UUID REFERENCES outreach(id),
    follow_up_scheduled_at TIMESTAMPTZ,
    response_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tenant_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL UNIQUE,
    auto_advance_on_approve BOOLEAN NOT NULL DEFAULT true,
    outreach_auto_send TEXT NOT NULL DEFAULT 'never',
    follow_up_enabled BOOLEAN NOT NULL DEFAULT false,
    follow_up_days INT NOT NULL DEFAULT 7,
    supplier_auto_select BOOLEAN NOT NULL DEFAULT false,
    email_provider TEXT NOT NULL DEFAULT 'sendgrid',
    email_from_name TEXT,
    email_from_address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Add channel_id to deals
ALTER TABLE deals ADD COLUMN channel_id UUID REFERENCES channels(id);

-- Indexes
CREATE INDEX idx_supplier_contacts_supplier ON supplier_contacts(supplier_id);
CREATE INDEX idx_supplier_notes_supplier ON supplier_notes(supplier_id);
CREATE INDEX idx_supplier_quotes_deal ON supplier_quotes(deal_id);
CREATE INDEX idx_supplier_quotes_supplier ON supplier_quotes(supplier_id);
CREATE INDEX idx_outreach_deal ON outreach(deal_id);
CREATE INDEX idx_outreach_supplier ON outreach(supplier_id);
CREATE INDEX idx_outreach_status ON outreach(tenant_id, status);
CREATE INDEX idx_channels_tenant ON channels(tenant_id);
```

---

## 11. Implementation Phases (within Phase 2)

To keep PRs focused:

| Sub-phase | What | Delivers |
|-----------|------|----------|
| 2A | Domain models + migrations + repos + basic supplier CRUD | Supplier list/detail pages |
| 2B | Outreach service + email provider adapter (SendGrid) + outreach flow | Draft, approve, send emails |
| 2C | Quote management + supplier selection + deal transition | Full sourcing → procuring flow |
| 2D | Tenant settings + automation levels | Configurable automation |
| 2E | Frontend pages (supplier list, detail, deal sourcing tab, settings) | Complete UI |

---

## 12. What's NOT in Phase 2

- Multi-channel implementation (Shopify, FB Marketplace, eBay) — Phase 4
- Validation/hybrid supplier workflows — future
- Third-party supplier matching APIs (SellerAssistant) — future, behind interface
- Procurement/PO management — Phase 3
- Email reply parsing/webhook handling — Phase 2B+ or follow-up
- Reverse sourcing from distributor catalogs — separate feature
