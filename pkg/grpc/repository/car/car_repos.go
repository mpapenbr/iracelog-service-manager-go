//nolint:whitespace,dupl // by design
package car

import (
	"context"

	carv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/car/v1"
	driverv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/driver/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository"
)

type persister struct {
	ctx            context.Context
	conn           repository.Querier
	eventID        int
	carLookup      map[uint32]int
	carClassLookup map[uint32]int
	entryLookup    map[uint32]int
	in             *racestatev1.PublishDriverDataRequest
}

func newPersister(
	conn repository.Querier,
	eventID int,
	in *racestatev1.PublishDriverDataRequest,
) *persister {
	return &persister{
		ctx:            context.Background(),
		conn:           conn,
		eventID:        eventID,
		in:             in,
		carLookup:      make(map[uint32]int),
		carClassLookup: make(map[uint32]int),
		entryLookup:    make(map[uint32]int),
	}
}

func (p *persister) persistCarClass() error {
	newCarClasses := p.newCarClasses()
	for i := range newCarClasses {
		c := newCarClasses[i]
		row := p.conn.QueryRow(p.ctx, `
	insert into c_car_class (
		event_id, name, car_class_id
	) values ($1,$2,$3)
	returning id
		`,
			p.eventID, c.Name, c.Id,
		)
		id := 0
		if err := row.Scan(&id); err != nil {
			return err
		}
		p.carClassLookup[c.Id] = id
	}
	return nil
}

func (p *persister) persistCar() error {
	newCars := p.newCars()
	for i := range newCars {
		c := newCars[i]
		row := p.conn.QueryRow(p.ctx, `
	insert into c_car (
		event_id, name, name_short, car_id, c_car_class_id, fuel_pct, power_adjust,
		weight_penalty, dry_tire_sets
	) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	returning id
		`,
			p.eventID, c.Name, c.NameShort, c.CarId,
			p.carClassLookup[uint32(c.CarClassId)], c.FuelPct, c.PowerAdjust,
			c.WeightPenalty, c.DryTireSets,
		)
		id := 0
		if err := row.Scan(&id); err != nil {
			return err
		}
		p.carLookup[c.CarId] = id
	}
	return nil
}

// returns a list of cars that are not yet in the database
func (p *persister) newCarClasses() []*carv1.CarClass {
	rows, err := p.conn.Query(p.ctx,
		`select id,car_class_id from c_car_class where event_id=$1`,
		p.eventID)
	if err != nil {
		return nil
	}

	for rows.Next() {
		var id, carClassID uint32
		if err := rows.Scan(&id, &carClassID); err != nil {
			return nil
		}
		p.carClassLookup[carClassID] = int(id)
	}

	ret := make([]*carv1.CarClass, 0, len(p.in.CarClasses))
	for i := range p.in.CarClasses {
		if _, ok := p.carClassLookup[p.in.CarClasses[i].Id]; !ok {
			ret = append(ret, p.in.CarClasses[i])
		}
	}
	return ret
}

// returns a list of cars that are not yet in the database
func (p *persister) newCars() []*carv1.CarInfo {
	rows, err := p.conn.Query(p.ctx, `select id,car_id from c_car where event_id=$1`,
		p.eventID)
	if err != nil {
		return nil
	}

	for rows.Next() {
		var id, carID uint32
		if err := rows.Scan(&id, &carID); err != nil {
			return nil
		}
		p.carLookup[carID] = int(id)
	}

	ret := make([]*carv1.CarInfo, 0, len(p.in.Cars))
	for i := range p.in.Cars {
		if _, ok := p.carLookup[p.in.Cars[i].CarId]; !ok {
			ret = append(ret, p.in.Cars[i])
		}
	}
	return ret
}

// returns a list of cars that are not yet in the database
func (p *persister) newEntries() []*carv1.CarEntry {
	rows, err := p.conn.Query(p.ctx,
		`select id, car_idx from c_car_entry where event_id=$1`,
		p.eventID)
	if err != nil {
		return nil
	}

	for rows.Next() {
		var id, carIdx uint32
		if err := rows.Scan(&id, &carIdx); err != nil {
			return nil
		}
		p.entryLookup[carIdx] = int(id)
	}

	ret := make([]*carv1.CarEntry, 0, len(p.in.Entries))
	for i := range p.in.Entries {
		if _, ok := p.entryLookup[p.in.Entries[i].Car.CarIdx]; !ok {
			ret = append(ret, p.in.Entries[i])
		}
	}
	return ret
}

// returns a list of cars that are not yet in the database
//
//nolint:lll // readability
func (p *persister) newDrivers(carEntryID int, drivers []*driverv1.Driver) []*driverv1.Driver {
	rows, err := p.conn.Query(p.ctx,
		`select id, driver_id from c_car_driver where c_car_entry_id=$1`,
		carEntryID)
	if err != nil {
		return nil
	}

	lookup := map[uint32]int{}
	for rows.Next() {
		var id, driverID uint32
		if err := rows.Scan(&id, &driverID); err != nil {
			return nil
		}
		lookup[driverID] = int(id)
	}

	ret := make([]*driverv1.Driver, 0, len(drivers))
	for i := range drivers {
		if _, ok := lookup[uint32(drivers[i].Id)]; !ok {
			ret = append(ret, drivers[i])
		}
	}
	return ret
}

//nolint:funlen // by design
func (p *persister) persistEntries() error {
	newEntries := p.newEntries()
	for i := range newEntries {
		c := newEntries[i]
		var err error
		// car entry
		row := p.conn.QueryRow(p.ctx, `
	insert into c_car_entry (
		event_id, c_car_id, car_idx, car_number, car_number_raw
	) values ($1,$2,$3,$4,$5)
	returning id
		`,
			p.eventID, p.carLookup[c.Car.CarId], c.Car.CarIdx,
			c.Car.CarNumber, c.Car.CarNumberRaw,
		)
		entryID := 0
		err = row.Scan(&entryID)
		if err != nil {
			return err
		}
		// team
		_, err = p.conn.Exec(p.ctx, `
	insert into c_car_team (
		c_car_entry_id,team_id,name
	) values ($1,$2,$3)
		`,
			entryID, c.Team.Id, c.Team.Name,
		)
		if err != nil {
			return err
		}
		p.entryLookup[c.Car.CarIdx] = entryID
	}
	// we need to check drivers for each entry
	for i := range p.in.Entries {
		c := p.in.Entries[i]
		// drivers
		newDrivers := p.newDrivers(p.entryLookup[c.Car.CarIdx], c.Drivers)
		for j := range newDrivers {
			d := newDrivers[j]
			_, err := p.conn.Exec(p.ctx, `
			insert into c_car_driver (
				c_car_entry_id, driver_id, name, initials, abbrev_name, irating,
				lic_level,lic_sub_level,lic_string
			) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			`, p.entryLookup[c.Car.CarIdx], d.Id, d.Name, d.Initials, d.AbbrevName,
				d.IRating, d.LicLevel, d.LicSubLevel, d.LicString,
			)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

// distributes the data from the proto message to several database tables
func Create(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
	driverstate *racestatev1.PublishDriverDataRequest,
) error {
	p := newPersister(conn, eventID, driverstate)
	if err := p.persistCarClass(); err != nil {
		return err
	}
	if err := p.persistCar(); err != nil {
		return err
	}
	if err := p.persistEntries(); err != nil {
		return err
	}
	return nil
}

func LoadByEventID(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
) (*racestatev1.PublishDriverDataRequest, error) {
	return nil, nil
}

// deletes an entry from the database, returns number of rows deleted.

//nolint:lll // readability
func DeleteByEventID(ctx context.Context, conn repository.Querier, eventID int) (int, error) {
	var err error
	var cmdTag pgconn.CommandTag
	_, err = conn.Exec(ctx, `
	delete from c_car_team where c_car_entry_id
	  in (select id from c_car_entry where event_id=$1)
	`, eventID)
	if err != nil {
		return 0, err
	}
	_, err = conn.Exec(ctx, `
	delete from c_car_driver where c_car_entry_id
	  in (select id from c_car_entry where event_id=$1)
	`, eventID)
	if err != nil {
		return 0, err
	}
	cmdTag, err = conn.Exec(ctx, "delete from c_car_entry where event_id=$1", eventID)
	if err != nil {
		return 0, err
	}

	_, err = conn.Exec(ctx, "delete from c_car where event_id=$1", eventID)
	if err != nil {
		return 0, err
	}
	_, err = conn.Exec(ctx, "delete from c_car_class where event_id=$1", eventID)
	if err != nil {
		return 0, err
	}

	return int(cmdTag.RowsAffected()), err
}
