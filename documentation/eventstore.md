# Plan for Implementing Event Store Infrastructure

Here is a step-by-step plan for implementing the event store, following a Test-Driven Development (TDD) approach.

---

### **Step 1: Create the Database Migration (Setup)**

This is the one step that comes before testing. We need the database table to exist before our tests can interact with it.

* **1A. Create the `up` migration:**
    * **File:** `migrations/000002_create_events_table.up.sql`
    * **Action:** Add the SQL to create the `events` table with columns for `event_id`, `aggregate_id`, `aggregate_type`, `event_type`, version, payload, and metadata. Include a unique constraint on `(aggregate_id, aggregate_version)` to prevent concurrency issues.

* **1B. Create the `down` migration:**
    * **File:** `migrations/000002_create_events_table.down.sql`
    * **Action:** Add the SQL to `DROP TABLE IF EXISTS events;`.

---

### **Step 2: Implement the Event Store Repository (TDD)**

Now, we follow the Red-Green-Refactor cycle.

* **2A. Write the First Test (Red):**
    * **File:** `internal/platform/eventstore/postgres_store_test.go`
    * **Action:** Create a test function `TestPostgresStore_Save`.
    * Inside the test:
        1.  Create a sample `EventEnvelope` from your `messaging` package.
        2.  Attempt to create an `eventstore.NewPostgresStore(...)`.
        3.  Call a `store.Save(ctx, envelope)` method.
        4.  This code will not compile. **This is your "Red" state.**

* **2B. Make the Test Compile and Pass (Green):**
    * **Files:** `internal/platform/eventstore/eventstore.go` and `postgres_store.go`
    * **Actions:**
        1.  Create the `eventstore.go` file and define the `Store` interface with a `Save` method.
        2.  Create the `postgres_store.go` file. Implement the `PostgresStore` struct and the `Save` method with the actual `INSERT` SQL logic.
        3.  Run the test. It should now compile and pass. **This is your "Green" state.**
        4.  In your test, add an assertion to query the database directly and verify the event was inserted correctly.

* **2C. Write a Test for Concurrency (Red):**
    * **File:** `internal/platform/eventstore/postgres_store_test.go`
    * **Action:** Add a new test, `TestPostgresStore_Save_ConcurrencyConflict`.
    * Inside the test:
        1.  Save an event for an aggregate with `version: 1`.
        2.  Attempt to save a *second* event for the *same aggregate* with `version: 1`.
        3.  Assert that the error returned from the second save is `eventstore.ErrConcurrencyConflict`.
        4.  This test will fail because your `Save` method doesn't return this specific error yet. **This is your "Red" state.**

* **2D. Refactor to Handle Concurrency (Green):**
    * **File:** `internal/platform/eventstore/postgres_store.go`
    * **Action:** Modify your `Save` method to check for the PostgreSQL unique constraint violation error (`23505`). If you see this error, wrap it and return your custom `ErrConcurrencyConflict`.
    * Run both tests. They should now both pass. **This is your final "Green" state.**

---

### **Step 3: Integrate into the Fabric Service (TDD)**

* **3A. Update the Service Test (Red):**
    * **File:** `internal/fabrics/application/fabric_command_service_test.go`
    * **Action:**
        1.  Create a `mockEventStore` that implements the `eventstore.Store` interface. It should have a `SavedCalled` flag.
        2.  In `TestFabricService_CreateFabric_HappyPath`, try to inject this mock into `NewFabricCommandService`.
        3.  Add the assertion `assert.True(t, mockEventStore.SavedCalled)`.
        4.  The code will fail to compile because the service doesn't know about the event store. **This is your "Red" state.**

* **3B. Update the Service Implementation (Green):**
    * **File:** `internal/fabrics/application/fabric_command_service.go`
    * **Action:**
        1.  Add the `eventStore` field to the `FabricService` struct.
        2.  Update `NewFabricCommandService` to accept and set the event store.
        3.  In `CreateFabric`, call `s.eventStore.Save()` *before* you call the publisher.
        4.  Run the test. It should now pass. **This is your "Green" state.**

---

### **Step 4: Bootstrap the Application (Final Integration)**

This final step is manual wiring, not TDD.

* **4A. Expose the DB Connection:**
    * **File:** `internal/bootstrap/repositories.go`
    * **Action:** Modify the `Repositories` struct to expose the raw `*sql.DB` pool.

* **4B. Wire Everything Together:**
    * **File:** `internal/bootstrap/services.go`
    * **Action:** In `NewServices`, create an instance of your `eventstore.NewPostgresStore` and pass it into the `fabricApp.NewFabricCommandService` constructor.