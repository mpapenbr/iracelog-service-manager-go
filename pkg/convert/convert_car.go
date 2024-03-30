package convert

import (
	carv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/car/v1"
	driverv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/driver/v1"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
)

func ConvertCarInfo(m *model.CarInfo) *carv1.CarInfo {
	return &carv1.CarInfo{
		CarId:         uint32(m.CarID),
		CarClassId:    int32(m.CarClassID),
		Name:          m.Name,
		NameShort:     m.NameShort,
		CarClassName:  m.CarClassName,
		FuelPct:       float32(m.FuelPct),
		PowerAdjust:   float32(m.PowerAdjust),
		WeightPenalty: float32(m.WeightPenalty),
		DryTireSets:   int32(m.DryTireSets),
	}
}

func ConvertCarInfos(m []model.CarInfo) []*carv1.CarInfo {
	ret := make([]*carv1.CarInfo, 0)
	for _, d := range m {
		ret = append(ret, ConvertCarInfo(&d))
	}
	return ret
}

func ConvertCarClass(m *model.CarClass) *carv1.CarClass {
	return &carv1.CarClass{
		Id:   uint32(m.ID),
		Name: m.Name,
	}
}

func ConvertCarClasses(m []model.CarClass) []*carv1.CarClass {
	ret := make([]*carv1.CarClass, 0)
	for _, d := range m {
		ret = append(ret, ConvertCarClass(&d))
	}
	return ret
}

func ConvertTeam(m *model.Team) *driverv1.Team {
	return &driverv1.Team{
		Id:   uint32(m.ID),
		Name: m.Name,
	}
}

func ConvertDriver(m *model.Driver) *driverv1.Driver {
	return &driverv1.Driver{
		Id:          int32(m.ID),
		Name:        m.Name,
		CarIdx:      uint32(m.CarIdx),
		IRating:     int32(m.IRating),
		Initials:    m.Initials,
		LicLevel:    int32(m.LicLevel),
		LicSubLevel: int32(m.LicSubLevel),
		LicString:   m.LicString,
		AbbrevName:  m.AbbrevName,
	}
}

func ConvertDrivers(m []model.Driver) []*driverv1.Driver {
	ret := make([]*driverv1.Driver, 0)
	for _, d := range m {
		ret = append(ret, ConvertDriver(&d))
	}
	return ret
}

func ConvertCurrentDrivers(m map[int]string) map[uint32]string {
	ret := make(map[uint32]string, 0)
	for k, v := range m {
		ret[uint32(k)] = v
	}
	return ret
}

func ConvertCarEntry(m *model.CarEntry) *carv1.CarEntry {
	return &carv1.CarEntry{
		Car:     ConvertCar(&m.Car),
		Team:    ConvertTeam(&m.Team),
		Drivers: ConvertDrivers(m.Drivers),
	}
}

func ConvertCarEntries(m []model.CarEntry) []*carv1.CarEntry {
	ret := make([]*carv1.CarEntry, 0)
	for _, d := range m {
		ret = append(ret, ConvertCarEntry(&d))
	}
	return ret
}

func ConvertCar(m *model.Car) *carv1.Car {
	return &carv1.Car{
		CarIdx:       uint32(m.CarIdx),
		CarId:        uint32(m.CarID),
		CarClassId:   int32(m.CarClassID),
		Name:         m.Name,
		CarNumber:    m.CarNumber,
		CarNumberRaw: int32(m.CarNumberRaw),
	}
}
