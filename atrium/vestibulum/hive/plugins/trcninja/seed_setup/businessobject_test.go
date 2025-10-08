package seed_setup

import "testing"

var BusinessObjectBenchmarkError error

func BenchmarkAddBusinessObject_erp_project_BusinessObject(b *testing.B) {
	var err error
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err = AddBusinessObject("my-kafka-group-")
		}
	})
	BusinessObjectBenchmarkError = err
}

func TestUpdateBusinessObject_erp_project_BusinessObject(t *testing.T) {
	t.Parallel()
	err := UpdateBusinessObject("my-kafka-group-")
	if err != nil {
		t.Errorf("Failed %v\n", err)
		t.Fail()
	}
}

func TestAddBusinessObject_erp_project_BusinessObject(t *testing.T) {
	t.Parallel()
	err := AddBusinessObject("my-kafka-group-")
	if err != nil {
		t.Errorf("Failed %v\n", err)
		t.Fail()
	}
}
