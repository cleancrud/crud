package model

import (
	"log"
	"os"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v4"
)

func PostgresTable(db, path, relative string, notint64 bool, dialect string) *Table {
	sqlstr, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	p, err := pg_query.Parse(string(sqlstr))
	if err != nil {
		log.Fatalln(err)
	}
	st := p.GetStmts()[0].GetStmt().GetCreateStmt()
	if st == nil {
		log.Fatalln("not have a create table stmt")
	}

	tableName := st.GetRelation().GetRelname()
	schemaName := st.GetRelation().GetSchemaname()
	gotableName := GoCamelCase(tableName)
	mytable := &Table{
		Database:    db,
		SchemaName:  schemaName,
		TableName:   tableName,
		GoTableName: gotableName,
		PackageName: strings.ToLower(gotableName),
		Dialect:     dialect,
	}
	columns, err := PostgresColumn(st, notint64)
	if err != nil {
		log.Fatal(err)
	}
	if len(columns) <= 0 {
		log.Fatal("schema or table not exist")
	}
	mytable.Fields = columns
	for _, v := range columns {
		if v.IsPrimaryKey {
			mytable.PrimaryKey = v
		}
		if v.GoColumnType == "time.Time" {
			mytable.ImportTime = true
		}
		v.ProtoType = GoTypeToProtoType(v.GoColumnType)
	}

	mytable.GenerateWhereCol = mytable.Fields
	mytable.RelativePath = relative

	return mytable
}

func PostgresColumn(ddl *pg_query.CreateStmt, notint64 bool) ([]*Column, error) {
	res := []*Column{}
	for k, vv := range ddl.GetTableElts() {
		v := vv.GetColumnDef()
		if v == nil {
			continue
		}
		var notNull bool
		var autoIncrement bool
		var primaryKey bool
		var comment string

		notNull = v.GetIsNotNull()
		var columnType string
		names := v.GetTypeName().GetNames()
		if len(names) < 1 {
			log.Fatalln("not type names")
		}
		columnType = names[len(names)-1].GetString_().GetSval()

		if strings.Contains(columnType, "serial") {
			autoIncrement = true
		}
		for _, v2 := range v.GetConstraints() {
			if v2.GetConstraint().Contype == pg_query.ConstrType_CONSTR_PRIMARY {
				primaryKey = true
			}
		}
		c := &Column{
			OrdinalPosition:           k,
			ColumnName:                v.GetColname(),
			DataType:                  columnType,
			ColumnType:                "",
			ColumnComment:             comment,
			NotNull:                   notNull,
			IsPrimaryKey:              primaryKey,
			IsAutoIncrment:            autoIncrement,
			IsDefaultCurrentTimestamp: false,
			GoColumnName:              "",
			GoColumnType:              "",
			BigType:                   0,
			GoConditionType:           "",
		}

		c.GoColumnName = GoCamelCase(c.ColumnName)
		c.GoColumnType, c.BigType = PostgresToGoFieldType(c.DataType, c.ColumnType)
		if strings.Contains(c.GoColumnType, "int") && !notint64 {
			c.GoColumnType = "int64"
		}
		c.GoConditionType = c.GoColumnType
		if c.BigType == bigtypeCompareTime {
			c.GoConditionType = "string"
		}
		res = append(res, c)

	}
	var primaryKey string
	for _, v := range ddl.Constraints {
		x := v.GetConstraint()
		if x.Contype == pg_query.ConstrType_CONSTR_PRIMARY {
			if len(x.Keys) > 1 {
				log.Fatal("primary key is not single column")
			}
			primaryKey = x.Keys[0].GetString_().GetSval()
		}

	}
	for _, v := range res {
		if v.ColumnName == primaryKey {
			v.IsPrimaryKey = true
		}
		v.ProtoType = GoTypeToProtoType(v.GoColumnType)
	}
	return res, nil
}

func PostgresToGoFieldType(dt, ct string) (string, int) {
	var unsigned bool
	if strings.Contains(ct, "unsigned") {
		unsigned = true
	}
	var typ string
	var gtp int
	switch dt {
	case "bit", "bit varying":
		typ = "[]byte"
		gtp = bigtypeCompareBit
	case "bool", "boolean":
		typ = "bool"
	case "char", "varchar", "character", "character varying":
		typ = "string"
		gtp = bigtypeCompareString
	case "text", "json":
		typ = "string"
	case "tinyint":
		typ = "int8"
		if unsigned {
			typ = "uint8"
		}
		gtp = bigtypeCompare
	case "smallint", "int2", "serial2", "smallserial":
		typ = "int16"
		if unsigned {
			typ = "uint16"
		}
		gtp = bigtypeCompare
	case "int4", "int", "integer", "serial4", "serial":
		typ = "int32"
		if unsigned {
			typ = "uint32"
		}
		gtp = bigtypeCompare
	case "bigint", "int8", "bigserial", "bigserial8":
		typ = "int64"
		if unsigned {
			typ = "uint64"
		}
		gtp = bigtypeCompare
	case "float", "real":
		typ = "float32"
		gtp = bigtypeCompare
	case "decimal", "double", "float8":
		typ = "float64"
		gtp = bigtypeCompare
	case "binary", "varbinary":
		typ = "[]byte"
		gtp = bigtypeCompare
	case "tinyblob", "blob", "mediumblob", "longblob":
		typ = "[]byte"
	case "timestamp", "datetime", "date":
		typ = "time.Time"
		gtp = bigtypeCompareTime
	case "time", "year", "enum", "set":
		typ = "string"
		gtp = bigtypeCompare
	default:
		typ = "UNKNOWN"
	}
	return typ, gtp
}
