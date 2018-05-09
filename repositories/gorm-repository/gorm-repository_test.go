package gormrepo

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/kucjac/jsonapi"
	"github.com/stretchr/testify/assert"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

var db *gorm.DB

type UserGORM struct {
	ID        uint       `jsonapi:"primary,users"`
	Name      string     `jsonapi:"attr,name"`
	Surname   string     `jsonapi:"attr,surname"`
	Pets      []*PetGORM `jsonapi:"relation,pets" gorm:"foreignkey:OwnerID"`
	CreatedAt time.Time  `jsonapi:"attr,created-at"`
}

type PetGORM struct {
	ID        uint      `jsonapi:"primary,pets"`
	Name      string    `jsonapi:"attr,name"`
	CreatedAt time.Time `jsonapi:"attr,created-at"`
	Owner     *UserGORM `jsonapi:"relation,owner"`
	OwnerID   uint      `jsonapi:"-"`
}

func TestGORMRepositoryGet(t *testing.T) {
	c, err := prepareJSONAPI(&UserGORM{}, &PetGORM{})
	if err != nil {
		t.Fatal(err)
	}
	defer clearDB()
	repo, err := prepareGORMRepo(&UserGORM{}, &PetGORM{})
	if err != nil {
		t.Fatal(err)
	}
	err = settleUsers(db)
	assert.Nil(t, err)

	req := httptest.NewRequest("GET", "/users/3?fields[users]=name,pets", nil)

	assert.NotNil(t, c.Models)
	scope, errs, err := c.BuildScopeSingle(req, &UserGORM{})
	assert.Nil(t, err)
	assert.Empty(t, errs)
	scope.NewValueSingle()
	dbErr := repo.Get(scope)
	assert.Nil(t, dbErr)

	req = httptest.NewRequest("GET", "/users/3?include=pets&fields[pets]=name", nil)

	scope, errs, _ = c.BuildScopeSingle(req, &UserGORM{})
	assert.Empty(t, errs)

	dbErr = repo.Get(scope)
	assert.Nil(t, dbErr)

	err = scope.SetIncludedPrimaries()
	assert.NoError(t, err)

	t.Log(scope.Value)

	for _, includedScope := range scope.IncludedScopes {
		if len(includedScope.IncludeValues) > 0 {
			t.Log(includedScope.PrimaryFilters[0].Values[0].Values)
			dbErr = repo.List(includedScope)
			assert.Nil(t, dbErr)
			manyIncludes := includedScope.Value.([]*PetGORM)
			t.Log(manyIncludes[0])
		} else {
			t.Log("No values")
		}

	}

}

func TestGORMRepositoryList(t *testing.T) {
	c, err := prepareJSONAPI(&UserGORM{}, &PetGORM{})
	if err != nil {
		t.Fatal(err)
	}
	defer clearDB()
	repo, err := prepareGORMRepo(&UserGORM{}, &PetGORM{})
	if err != nil {
		t.Fatal(err)
	}
	assert.Nil(t, settleUsers(repo.db))

	req := httptest.NewRequest("GET", "/users?fields[users]=name,surname,pets", nil)
	scope, errs, err := c.BuildScopeList(req, &UserGORM{})
	assert.Nil(t, err)
	assert.Empty(t, errs)

	dbErr := repo.List(scope)
	assert.Nil(t, dbErr)

	req = httptest.NewRequest("GET", "/pets?fields[pets]=name,owner", nil)
	scope, errs, _ = c.BuildScopeList(req, &PetGORM{})
	assert.Empty(t, errs)

	dbErr = repo.List(scope)
	assert.Nil(t, dbErr)

	req = httptest.NewRequest("GET", "/pets?include=owner", nil)
	scope, _, _ = c.BuildScopeList(req, &PetGORM{})

	dbErr = repo.List(scope)
	assert.Nil(t, dbErr)

	err = scope.SetIncludedPrimaries()

	assert.NoError(t, err)

	for _, includedScope := range scope.IncludedScopes {
		dbErr = repo.List(includedScope)
		assert.Nil(t, dbErr)

	}

	many, ok := scope.Value.([]*PetGORM)
	assert.True(t, ok)

	for _, single := range many {
		t.Log(single)
	}

	manyU, ok := scope.IncludedScopes[c.MustGetModelStruct(&UserGORM{})].Value.([]*UserGORM)
	assert.True(t, ok)

	t.Log("Includes!")
	for _, single := range manyU {
		t.Log(single)
	}

}

func prepareJSONAPI(models ...interface{}) (*jsonapi.Controller, error) {
	c := jsonapi.New()
	err := c.PrecomputeModels(models...)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func prepareGORMRepo(models ...interface{}) (*GORMRepository, error) {
	var err error
	db, err = gorm.Open("sqlite3", "test.db")
	if err != nil {
		return nil, err
	}
	db.Debug()
	db.AutoMigrate(models...)
	repo, err := New(db)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

func clearDB() error {
	err := db.Close()
	if err != nil {
		return err
	}
	// os.IsPermission(err)
	// return nil
	return os.Remove("test.db")
}

func settleUsers(db *gorm.DB) error {
	var users []*UserGORM = []*UserGORM{
		{ID: 1, Name: "Zygmunt", Surname: "Waza", Pets: []*PetGORM{{ID: 1, Name: "Maniek"}}},
		{ID: 2, Name: "Mathew", Surname: "Kovalsky"},
		{ID: 3, Name: "Jules", Surname: "Ceasar", Pets: []*PetGORM{{ID: 2, Name: "Cerberus"}}},
		{ID: 4, Name: "Napoleon", Surname: "Bonaparte", Pets: []*PetGORM{{Name: "Boatswain"}}},
	}
	for _, u := range users {
		err := db.Create(&u).Error
		if err != nil {
			return err
		}
	}
	return nil
}
