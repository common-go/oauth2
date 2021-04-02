package oauth2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type SqlConfigurationRepository struct {
	DB                     *sql.DB
	TableName              string
	OAuth2UserRepositories map[string]OAuth2UserRepository
	Status                 string
	Active                 string
	Driver                 string
}

func NewSqlConfigurationRepository(db *sql.DB, tableName string, oAuth2PersonInfoServices map[string]OAuth2UserRepository, status string, active string) *SqlConfigurationRepository {
	if len(status) == 0 {
		status = "status"
	}
	if len(active) == 0 {
		active = "A"
	}

	return &SqlConfigurationRepository{db, tableName, oAuth2PersonInfoServices, status, active, GetDriver(db)}
}

func (s *SqlConfigurationRepository) GetConfiguration(ctx context.Context, id string) (*Configuration, string, error) {
	model := Configuration{}
	limitRowsQL := "limit 1"
	driver := GetDriver(s.DB)
	if driver == DriverOracle {
		limitRowsQL = "and rownum = 1"
	}
	query := fmt.Sprintf(`select * from %s where %s = %s %s`, s.TableName, "id", BuildParam(0, GetDriver(s.DB)), limitRowsQL)
	rows, err := s.DB.Query(query, id)
	err2 := ScanRow(rows, &model)
	if err2 == sql.ErrNoRows {
		return nil, "", err2
	}
	clientId := model.ClientId
	clientId, err = s.OAuth2UserRepositories[id].GetRequestTokenOAuth(ctx, model.ClientId, model.ClientSecret)
	return &model, clientId, err
}

func (s *SqlConfigurationRepository) GetConfigurations(ctx context.Context) (*[]Configuration, error) {
	query := fmt.Sprintf(`select * from %s where %s = %s `, s.TableName, s.Status, BuildParam(0, s.Driver))
	rows, err := s.DB.Query(query, s.Active)
	if err != nil {
		return nil, err
	}
	model := Configuration{}
	models := make([]Configuration, 0)
	modelType := reflect.TypeOf(model)
	fieldsIndex, er1 := GetColumnIndexes(modelType)
	if er1 != nil {
		return nil, er1
	}
	defer rows.Close()
	err1 := Scans(rows, &models, fieldsIndex)
	if err1 != nil {
		return nil, err1
	}
	return &models, err
}

// StructScan : transfer struct to slice for scan
func StructScanByIndex(s interface{}, fieldsIndex map[string]int, columns []string) (r []interface{}) {
	if s != nil {
		maps := reflect.Indirect(reflect.ValueOf(s))
		fieldsIndexSelected := make([]int, 0)
		for _, columnsName := range columns {
			columnsName = strings.ToLower(columnsName)
			if index, ok := fieldsIndex[columnsName]; ok {
				fieldsIndexSelected = append(fieldsIndexSelected, index)
				r = append(r, maps.Field(index).Addr().Interface())
			} else {
				var ignore interface{}
				r = append(r, &ignore)
			}
		}
	}
	return
}

func Scans(rows *sql.Rows, results interface{}, fieldsIndex map[string]int) (err error) {
	columns, er0 := rows.Columns()
	if er0 != nil {
		return er0
	}
	modelType := reflect.TypeOf(results).Elem().Elem()
	for rows.Next() {
		initModel := reflect.New(modelType).Interface()
		if err = rows.Scan(StructScanByIndex(initModel, fieldsIndex, columns)...); err == nil {
			appendToArray(results, initModel)
		}
	}
	return
}

func ScanRow(rows *sql.Rows, result interface{}) (err error) {
	columns, er0 := rows.Columns()
	if er0 != nil {
		return er0
	}
	modelType := reflect.TypeOf(result).Elem()
	fieldsIndex, er0 := GetColumnIndexes(modelType)
	if er0 != nil {
		return er0
	}
	for rows.Next() {
		if err = rows.Scan(StructScanByIndex(result, fieldsIndex, columns)...); err == nil {
		}
		break
	}
	return
}

func appendToArray(arr interface{}, item interface{}) {
	arrValue := reflect.ValueOf(arr)
	elemValue := reflect.Indirect(arrValue)
	itemValue := reflect.ValueOf(item)
	if itemValue.Kind() == reflect.Ptr {
		itemValue = reflect.Indirect(itemValue)
	}
	elemValue.Set(reflect.Append(elemValue, itemValue))
}

func GetDriver(db *sql.DB) string {
	if db == nil {
		return DriverNotSupport
	}
	driver := reflect.TypeOf(db.Driver()).String()
	switch driver {
	case "*pq.Driver":
		return DriverPostgres
	case "*mysql.MySQLDriver":
		return DriverMysql
	case "*mssql.Driver":
		return DriverMssql
	case "*godror.drv":
		return DriverOracle
	default:
		return DriverNotSupport
	}
}

func GetColumnIndexes(modelType reflect.Type) (map[string]int, error) {
	mapp := make(map[string]int, 0)
	if modelType.Kind() != reflect.Struct {
		return mapp, errors.New("bad type")
	}
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		ormTag := field.Tag.Get("gorm")
		column, ok := FindTag(ormTag, "column")
		if ok {
			mapp[column] = i
		}
	}
	return mapp, nil
}

func FindTag(tag string, key string) (string, bool) {
	if has := strings.Contains(tag, key); has {
		str1 := strings.Split(tag, ";")
		num := len(str1)
		for i := 0; i < num; i++ {
			str2 := strings.Split(str1[i], ":")
			for j := 0; j < len(str2); j++ {
				if str2[j] == key {
					return str2[j+1], true
				}
			}
		}
	}
	return "", false
}