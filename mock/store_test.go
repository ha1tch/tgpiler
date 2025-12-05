package mock

import (
	"context"
	"testing"
)

func TestMockStore_Insert(t *testing.T) {
	store := NewMockStore()
	
	id, err := store.Insert("Users", map[string]interface{}{
		"Username": "test_user",
		"Email":    "test@example.com",
	})
	
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	
	if id != 1 {
		t.Errorf("Expected ID 1, got %d", id)
	}
	
	// Insert another
	id2, _ := store.Insert("Users", map[string]interface{}{
		"Username": "test_user2",
	})
	
	if id2 != 2 {
		t.Errorf("Expected ID 2, got %d", id2)
	}
}

func TestMockStore_Select(t *testing.T) {
	store := SetupTestStore()
	
	// Select all users
	users, err := store.Select("Users", nil)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	
	if len(users) != 3 {
		t.Errorf("Expected 3 users, got %d", len(users))
	}
	
	// Select with filter
	activeUsers, _ := store.Select("Users", map[string]interface{}{
		"IsActive": true,
	})
	
	if len(activeUsers) != 2 {
		t.Errorf("Expected 2 active users, got %d", len(activeUsers))
	}
}

func TestMockStore_SelectOne(t *testing.T) {
	store := SetupTestStore()
	
	user, err := store.SelectOne("Users", map[string]interface{}{
		"Username": "john_doe",
	})
	
	if err != nil {
		t.Fatalf("SelectOne failed: %v", err)
	}
	
	if user == nil {
		t.Fatal("Expected user, got nil")
	}
	
	if user["Email"] != "john@example.com" {
		t.Errorf("Expected john@example.com, got %v", user["Email"])
	}
}

func TestMockStore_Update(t *testing.T) {
	store := SetupTestStore()
	
	affected, err := store.Update("Users",
		map[string]interface{}{"IsActive": false},
		map[string]interface{}{"Username": "john_doe"},
	)
	
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	
	if affected != 1 {
		t.Errorf("Expected 1 affected row, got %d", affected)
	}
	
	// Verify update
	user, _ := store.SelectOne("Users", map[string]interface{}{"Username": "john_doe"})
	if user["IsActive"] != false {
		t.Error("User should be inactive")
	}
}

func TestMockStore_Delete(t *testing.T) {
	store := SetupTestStore()
	
	// Count before
	before, _ := store.Select("Users", nil)
	
	deleted, err := store.Delete("Users", map[string]interface{}{
		"Username": "bob_smith",
	})
	
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	
	if deleted != 1 {
		t.Errorf("Expected 1 deleted row, got %d", deleted)
	}
	
	// Count after
	after, _ := store.Select("Users", nil)
	
	if len(after) != len(before)-1 {
		t.Errorf("Expected %d users after delete, got %d", len(before)-1, len(after))
	}
}

func TestMockStore_Truncate(t *testing.T) {
	store := SetupTestStore()
	
	err := store.Truncate("Users")
	if err != nil {
		t.Fatalf("Truncate failed: %v", err)
	}
	
	users, _ := store.Select("Users", nil)
	if len(users) != 0 {
		t.Errorf("Expected 0 users after truncate, got %d", len(users))
	}
}

func TestMockStore_Clear(t *testing.T) {
	store := SetupTestStore()
	
	store.Clear()
	
	users, _ := store.Select("Users", nil)
	orders, _ := store.Select("Orders", nil)
	
	if len(users) != 0 || len(orders) != 0 {
		t.Error("Clear should remove all data")
	}
}

func TestGenericService_Select(t *testing.T) {
	store := SetupTestStore()
	service := NewGenericService(store)
	
	resp, err := service.Execute(context.Background(), "SELECT", GenericRequest{
		Table: "Users",
		Where: map[string]interface{}{"IsActive": true},
	})
	
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	
	if !resp.Success {
		t.Errorf("Expected success, got error: %s", resp.Error)
	}
	
	if len(resp.Records) != 2 {
		t.Errorf("Expected 2 records, got %d", len(resp.Records))
	}
}

func TestGenericService_Insert(t *testing.T) {
	store := NewMockStore()
	service := NewGenericService(store)
	
	resp, err := service.Execute(context.Background(), "INSERT", GenericRequest{
		Table: "Products",
		Data: map[string]interface{}{
			"SKU":   "NEW-001",
			"Name":  "New Product",
			"Price": 19.99,
		},
	})
	
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	
	if !resp.Success {
		t.Errorf("Expected success, got error: %s", resp.Error)
	}
	
	if resp.InsertedID != 1 {
		t.Errorf("Expected InsertedID 1, got %d", resp.InsertedID)
	}
}

func TestGenericService_Update(t *testing.T) {
	store := SetupTestStore()
	service := NewGenericService(store)
	
	resp, err := service.Execute(context.Background(), "UPDATE", GenericRequest{
		Table: "Products",
		Set:   map[string]interface{}{"Price": 39.99},
		Where: map[string]interface{}{"SKU": "PROD-001"},
	})
	
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	
	if !resp.Success {
		t.Errorf("Expected success, got error: %s", resp.Error)
	}
	
	if resp.AffectedRows != 1 {
		t.Errorf("Expected 1 affected row, got %d", resp.AffectedRows)
	}
}

func TestGenericService_Delete(t *testing.T) {
	store := SetupTestStore()
	service := NewGenericService(store)
	
	resp, err := service.Execute(context.Background(), "DELETE", GenericRequest{
		Table: "Orders",
		Where: map[string]interface{}{"Status": "Pending"},
	})
	
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	
	if !resp.Success {
		t.Errorf("Expected success, got error: %s", resp.Error)
	}
	
	if resp.AffectedRows != 1 {
		t.Errorf("Expected 1 deleted row, got %d", resp.AffectedRows)
	}
}

func TestGenericService_Pagination(t *testing.T) {
	store := SetupTestStore()
	service := NewGenericService(store)
	
	// Get first 2
	resp, _ := service.Execute(context.Background(), "SELECT", GenericRequest{
		Table: "Users",
		Limit: 2,
	})
	
	if len(resp.Records) != 2 {
		t.Errorf("Expected 2 records with limit, got %d", len(resp.Records))
	}
	
	// Get with offset
	resp, _ = service.Execute(context.Background(), "SELECT", GenericRequest{
		Table:  "Users",
		Offset: 1,
		Limit:  2,
	})
	
	if len(resp.Records) != 2 {
		t.Errorf("Expected 2 records with offset, got %d", len(resp.Records))
	}
}

func TestSetupTestStore(t *testing.T) {
	store := SetupTestStore()
	
	users := store.GetAllRecords("Users")
	orders := store.GetAllRecords("Orders")
	products := store.GetAllRecords("Products")
	
	if len(users) != 3 {
		t.Errorf("Expected 3 users, got %d", len(users))
	}
	
	if len(orders) != 3 {
		t.Errorf("Expected 3 orders, got %d", len(orders))
	}
	
	if len(products) != 3 {
		t.Errorf("Expected 3 products, got %d", len(products))
	}
}
