//nolint:whitespace,lll // readability
package car

import (
	"context"
	"testing"

	carv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/car/v1"
	driverv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/driver/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"

	base "github.com/mpapenbr/iracelog-service-manager-go/testsupport/basedata"
	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

func sampleRequestData() *racestatev1.PublishDriverDataRequest {
	return &racestatev1.PublishDriverDataRequest{
		SessionTime: 1000,
		Cars: []*carv1.CarInfo{
			{Name: "car1", NameShort: "shortCar1", CarId: 1, CarClassId: 12, FuelPct: 1, PowerAdjust: -0.1, WeightPenalty: 10, DryTireSets: 4},
		},
		CarClasses: []*carv1.CarClass{
			{Name: "carClass1", Id: 12},
			{Name: "carClass2", Id: 34},
		},
		Entries: []*carv1.CarEntry{
			{
				Car:  &carv1.Car{CarIdx: 1, CarId: 1, Name: "car1", CarNumber: "10", CarNumberRaw: 10, CarClassId: 12},
				Team: &driverv1.Team{Id: 901, Name: "team1"},
				Drivers: []*driverv1.Driver{
					{Id: 991, Name: "driver1", CarIdx: 1, IRating: 1000, Initials: "D1", AbbrevName: "AbbrD1", LicLevel: 24, LicSubLevel: 413, LicString: "P 4.13"},
					{Id: 992, Name: "driver2", CarIdx: 1, IRating: 2000, Initials: "D2", AbbrevName: "AbbrD2", LicLevel: 23, LicSubLevel: 423, LicString: "A 4.23"},
				},
			},
		},
		Timestamp: base.TestTime(),
	}
}

func TestCreate(t *testing.T) {
	pool := testdb.InitTestDb()
	event := base.CreateSampleEvent(pool)
	var err error
	req := sampleRequestData()
	err = Create(context.Background(), pool, int(event.Id), req)
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}
}

func TestAddToExisting(t *testing.T) {
	pool := testdb.InitTestDb()
	event := base.CreateSampleEvent(pool)
	var err error
	req := sampleRequestData()
	err = Create(context.Background(), pool, int(event.Id), req)
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}
	req.Cars = append(req.Cars, &carv1.CarInfo{Name: "car2", NameShort: "shortCar2", CarId: 2, CarClassId: 34})
	req.CarClasses = append(req.CarClasses, &carv1.CarClass{Name: "carClass3", Id: 56})
	req.Entries = []*carv1.CarEntry{
		{
			Car:  &carv1.Car{CarIdx: 1, CarId: 1, Name: "car1", CarNumber: "10", CarNumberRaw: 10, CarClassId: 12},
			Team: &driverv1.Team{Id: 901, Name: "team1"},
			Drivers: []*driverv1.Driver{
				{Id: 991, Name: "driver1", CarIdx: 1, IRating: 1000, Initials: "D1", AbbrevName: "AbbrD1", LicLevel: 24, LicSubLevel: 413, LicString: "P 4.13"},
				{Id: 992, Name: "driver2", CarIdx: 1, IRating: 2000, Initials: "D2", AbbrevName: "AbbrD2", LicLevel: 23, LicSubLevel: 423, LicString: "A 4.23"},
				{Id: 993, Name: "driver3", CarIdx: 1, IRating: 3000, Initials: "D3", AbbrevName: "AbbrD3", LicLevel: 24, LicSubLevel: 413, LicString: "P 4.13"},
			},
		},
		{
			Car:  &carv1.Car{CarIdx: 2, CarId: 2, Name: "car2", CarNumber: "20", CarNumberRaw: 20, CarClassId: 56},
			Team: &driverv1.Team{Id: 902, Name: "team2"},
			Drivers: []*driverv1.Driver{
				{Id: 994, Name: "driver10", CarIdx: 2, IRating: 1000, Initials: "D10", AbbrevName: "AbbrD10", LicLevel: 24, LicSubLevel: 413, LicString: "P 4.13"},
			},
		},
	}

	err = Create(context.Background(), pool, int(event.Id), req)
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}
}

func TestDelete(t *testing.T) {
	pool := testdb.InitTestDb()
	event := base.CreateSampleEvent(pool)
	var err error
	err = Create(context.Background(), pool, int(event.Id), sampleRequestData())
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}
	num, err := DeleteByEventId(context.Background(), pool, int(event.Id))
	if err != nil {
		t.Errorf("DeleteByEventId() error = %v", err)
	}
	if num != 1 {
		t.Errorf("DeleteByEventId() = %v, want 1", num)
	}
}
