package db

import "testing"

func TestRelationQuotesSchemaAndTableSeparately(t *testing.T) {
	relation := Relation{Schema: `Team "Blue"`, Name: `Order.Items`}
	if got := relation.QualifiedName(); got != `"Team ""Blue"""."Order.Items"` {
		t.Fatalf("qualified name = %q", got)
	}
	if got := relation.DisplayName(); got != `Team "Blue".Order.Items` {
		t.Fatalf("display name = %q", got)
	}
	for _, invalid := range []Relation{{Name: "orders"}, {Schema: "public"}, {Schema: " ", Name: "orders"}} {
		if err := invalid.Validate(); err == nil {
			t.Fatalf("invalid relation accepted: %#v", invalid)
		}
	}
}

func TestTableCursorIsScopedAndCloned(t *testing.T) {
	relation := Relation{Schema: "public", Name: "orders"}
	cursor := &TableCursor{
		Values:       []CellValue{{Raw: []byte("abc"), DatabaseTypeName: "TEXT", Text: "abc"}},
		OrderColumns: []string{"id"},
		ConnectionID: 7,
		Relation:     relation,
		SortColumn:   "id",
		FilterColumn: "status",
		FilterValue:  "paid",
	}
	request := TableBrowseRequest{
		ConnectionID: 7, SortColumn: "id", FilterColumn: "status", FilterValue: "paid", Cursor: cursor,
	}
	if err := cursor.validateScope(relation, request, []string{"id"}); err != nil {
		t.Fatalf("matching cursor rejected: %v", err)
	}

	mutations := []func(*TableBrowseRequest){
		func(value *TableBrowseRequest) { value.ConnectionID++ },
		func(value *TableBrowseRequest) { value.SortColumn = "created_at" },
		func(value *TableBrowseRequest) { value.Descending = true },
		func(value *TableBrowseRequest) { value.FilterColumn = "customer_id" },
		func(value *TableBrowseRequest) { value.FilterValue = "pending" },
	}
	for index, mutate := range mutations {
		changed := request
		mutate(&changed)
		if err := cursor.validateScope(relation, changed, []string{"id"}); err == nil {
			t.Fatalf("scope mutation %d was accepted", index)
		}
	}
	if err := cursor.validateScope(Relation{Schema: "archive", Name: "orders"}, request, []string{"id"}); err == nil {
		t.Fatal("cursor was accepted for another relation")
	}

	clone := cursor.Clone()
	clone.Values[0].Raw.([]byte)[0] = 'z'
	clone.OrderColumns[0] = "changed"
	if clone == cursor {
		t.Fatal("cursor clone reused the original pointer")
	}
	if string(cursor.Values[0].Raw.([]byte)) != "abc" || cursor.OrderColumns[0] != "id" {
		t.Fatal("cursor clone reused the original raw byte slice")
	}
}
