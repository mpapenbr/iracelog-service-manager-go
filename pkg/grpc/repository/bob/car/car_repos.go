//nolint:whitespace,dupl // by design
package car

import (
	"context"

	carv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/car/v1"
	driverv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/driver/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/aarondl/opt/omit"
	"github.com/shopspring/decimal"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/models"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	bobCtx "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/context"
)

type (
	repo struct {
		conn bob.Executor
	}
)

var _ api.CarRepository = (*repo)(nil)

func NewCarRepository(conn bob.Executor) api.CarRepository {
	return &repo{
		conn: conn,
	}
}

type persister struct {
	ctx            context.Context
	conn           bob.Executor
	eventID        int
	carLookup      map[uint32]int32
	carClassLookup map[uint32]int32
	entryLookup    map[uint32]int32
	in             *racestatev1.PublishDriverDataRequest
}

func newPersister(
	ctx context.Context,
	conn bob.Executor,
	eventID int,
	in *racestatev1.PublishDriverDataRequest,
) *persister {
	return &persister{
		ctx:            ctx,
		conn:           conn,
		eventID:        eventID,
		in:             in,
		carLookup:      make(map[uint32]int32),
		carClassLookup: make(map[uint32]int32),
		entryLookup:    make(map[uint32]int32),
	}
}

func (p *persister) persistCarClass() error {
	newCarClasses := p.newCarClasses()
	for i := range newCarClasses {
		c := newCarClasses[i]
		setter := &models.CCarClassSetter{
			EventID:    omit.From(int32(p.eventID)),
			Name:       omit.From(c.Name),
			CarClassID: omit.From(int32(c.Id)),
		}
		if res, err := models.CCarClasses.Insert(
			setter,
		).One(p.ctx, p.conn); err == nil {
			p.carClassLookup[c.Id] = res.ID
		} else {
			return err
		}
	}
	return nil
}

func (p *persister) persistCar() error {
	newCars := p.newCars()
	for i := range newCars {
		c := newCars[i]
		setter := &models.CCarSetter{
			EventID:       omit.From(int32(p.eventID)),
			Name:          omit.From(c.Name),
			NameShort:     omit.From(c.NameShort),
			CarID:         omit.From(int32(c.CarId)),
			CCarClassID:   omit.From(p.carClassLookup[uint32(c.CarClassId)]),
			FuelPCT:       omit.From(decimal.NewFromFloat32(c.FuelPct)),
			PowerAdjust:   omit.From(decimal.NewFromFloat32(c.PowerAdjust)),
			WeightPenalty: omit.From(decimal.NewFromFloat32(c.WeightPenalty)),
			DryTireSets:   omit.From(c.DryTireSets),
		}
		if res, err := models.CCars.Insert(
			setter,
		).One(p.ctx, p.conn); err == nil {
			p.carLookup[c.CarId] = res.ID
		} else {
			return err
		}
	}
	return nil
}

// returns a list of cars that are not yet in the database
func (p *persister) newCarClasses() []*carv1.CarClass {
	res, err := models.CCarClasses.Query(
		models.SelectWhere.CCarClasses.EventID.EQ(int32(p.eventID)),
	).All(p.ctx, p.conn)
	if err != nil {
		return nil
	}

	for i := range res {
		p.carClassLookup[uint32(res[i].CarClassID)] = res[i].ID
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
	res, err := models.CCars.Query(
		models.SelectWhere.CCars.EventID.EQ(int32(p.eventID)),
	).All(p.ctx, p.conn)
	if err != nil {
		return nil
	}

	for i := range res {
		p.carLookup[uint32(res[i].CarID)] = res[i].ID
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
	res, err := models.CCarEntries.Query(
		models.SelectWhere.CCarEntries.EventID.EQ(int32(p.eventID)),
	).All(p.ctx, p.conn)
	if err != nil {
		return nil
	}

	for i := range res {
		p.entryLookup[uint32(res[i].CarIdx)] = res[i].ID
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
func (p *persister) newDrivers(carEntryID int32, drivers []*driverv1.Driver) []*driverv1.Driver {
	res, err := models.CCarDrivers.Query(
		models.SelectWhere.CCarDrivers.CCarEntryID.EQ(carEntryID),
	).All(p.ctx, p.conn)
	if err != nil {
		return nil
	}
	lookup := map[uint32]int{}
	for i := range res {
		lookup[uint32(res[i].DriverID)] = int(res[i].ID)
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
		ceSetter := &models.CCarEntrySetter{
			EventID:      omit.From(int32(p.eventID)),
			CCarID:       omit.From(p.carLookup[c.Car.CarId]),
			CarIdx:       omit.From(int32(c.Car.CarIdx)),
			CarNumber:    omit.From(c.Car.CarNumber),
			CarNumberRaw: omit.From(c.Car.CarNumberRaw),
		}
		var ceID int32
		var res *models.CCarEntry

		res, err = models.CCarEntries.Insert(
			ceSetter,
		).One(p.ctx, p.conn)
		if err != nil {
			return err
		}
		ceID = res.ID

		// team
		ctSetter := &models.CCarTeamSetter{
			CCarEntryID: omit.From(ceID),
			TeamID:      omit.From(int32(c.Team.Id)),
			Name:        omit.From(c.Team.Name),
		}
		if _, err = models.CCarTeams.Insert(
			ctSetter,
		).Exec(p.ctx, p.conn); err != nil {
			return err
		}
		p.entryLookup[c.Car.CarIdx] = ceID
	}
	// we need to check drivers for each entry
	for i := range p.in.Entries {
		c := p.in.Entries[i]
		// drivers
		newDrivers := p.newDrivers(p.entryLookup[c.Car.CarIdx], c.Drivers)
		for j := range newDrivers {
			d := newDrivers[j]
			cdSetter := &models.CCarDriverSetter{
				CCarEntryID: omit.From(p.entryLookup[c.Car.CarIdx]),
				DriverID:    omit.From(d.Id),
				Name:        omit.From(d.Name),
				Initials:    omit.From(d.Initials),
				AbbrevName:  omit.From(d.AbbrevName),
				Irating:     omit.From(d.IRating),
				LicLevel:    omit.From(d.LicLevel),
				LicSubLevel: omit.From(d.LicSubLevel),
				LicString:   omit.From(d.LicString),
			}
			if _, err := models.CCarDrivers.Insert(
				cdSetter,
			).Exec(p.ctx, p.conn); err != nil {
				return err
			}
		}

	}
	return nil
}

// distributes the data from the proto message to several database tables
func (r *repo) Create(
	ctx context.Context,
	eventID int,
	driverstate *racestatev1.PublishDriverDataRequest,
) error {
	p := newPersister(ctx, r.getExecutor(ctx), eventID, driverstate)
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

// deletes all car related entries of an event from the database
// returns number of rows deleted.
func (r *repo) DeleteByEventID(ctx context.Context, eventID int) (int, error) {
	var err error
	var num int64

	subQuery := psql.Select(
		sm.Columns(models.CCarEntryColumns.ID),
		sm.From(models.TableNames.CCarEntries),
		models.SelectWhere.CCarEntries.EventID.EQ(int32(eventID)),
	)

	_, err = models.CCarTeams.Delete(
		dm.Where(models.CCarTeamColumns.CCarEntryID.In(subQuery))).
		Exec(ctx, r.getExecutor(ctx))
	if err != nil {
		return 0, err
	}
	_, err = models.CCarDrivers.Delete(
		dm.Where(models.CCarDriverColumns.CCarEntryID.In(subQuery))).
		Exec(ctx, r.getExecutor(ctx))
	if err != nil {
		return 0, err
	}
	num, err = models.CCarEntries.Delete(
		models.DeleteWhere.CCarEntries.EventID.EQ(int32(eventID)),
	).Exec(ctx, r.getExecutor(ctx))
	if err != nil {
		return 0, err
	}
	_, err = models.CCars.Delete(
		models.DeleteWhere.CCars.EventID.EQ(int32(eventID)),
	).Exec(ctx, r.getExecutor(ctx))
	if err != nil {
		return 0, err
	}
	_, err = models.CCarClasses.Delete(
		models.DeleteWhere.CCarClasses.EventID.EQ(int32(eventID)),
	).Exec(ctx, r.getExecutor(ctx))
	if err != nil {
		return 0, err
	}
	return int(num), err
}

func (r *repo) getExecutor(ctx context.Context) bob.Executor {
	if executor := bobCtx.FromContext(ctx); executor != nil {
		return executor
	}
	return r.conn
}
