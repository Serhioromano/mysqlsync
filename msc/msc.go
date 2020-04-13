package msc

import (
	"database/sql"
	// "encoding/json"
	"fmt"
	"reflect"
	// "github.com/go-sql-driver/mysql"
)

// DBConn DBConnecition
type DBConn struct {
	Conn   *sql.DB
	Prefix string
	Scheme string
}

// SQLConnect connect to DB
func (d *DBConn) SQLConnect(dsn string) error {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	d.Conn = conn
	return nil
}

type MSCConfig struct {
	User        string
	Pass        string
	Host        string
	Port        string
	DB          string
	FilesPath   string
	File        string
	Prefix      string
	DTable      bool
	DColumn     bool
	DIndex      bool
	DConstraint bool
}

// Table table structure
type Table struct {
	Name          string  `json:"Name"`
	Engine        string  `json:"Engine"`
	Version       int     `json:"Version"`
	RowFormat     string  `json:"Row_format"`
	Rows          int     `json:"Rows"`
	AvgRowLength  int     `json:"Avg_row_length"`
	DataLength    int     `json:"Data_length"`
	MaxDataLength int     `json:"Max_data_length"`
	IndexLength   int     `json:"Index_length"`
	DataFree      int     `json:"Data_free"`
	AutoIncrement *string `json:"Auto_increment"`
	CreateTime    *string `json:"Create_time"`
	UpdateTime    *string `json:"Update_time"`
	CheckTime     *string `json:"Check_time"`
	Collation     string  `json:"Collation"`
	Checksum      *string `json:"Checksum"`
	CreateOptions string  `json:"Create_options"`
	Comment       string  `json:"Comment"`
}

// GetTables get list of tables in a DB
func (d *DBConn) GetTables() ([]Table, error) {
	result, err := d.Conn.Query(fmt.Sprintf("SHOW TABLE STATUS LIKE '%s%%'", d.Prefix))

	var out []Table
	if err != nil {
		return out, err
	}

	for result.Next() {
		var tbl = Table{}
		err = result.Scan(
			&tbl.Name,
			&tbl.Engine,
			&tbl.Version,
			&tbl.RowFormat,
			&tbl.Rows,
			&tbl.AvgRowLength,
			&tbl.DataLength,
			&tbl.MaxDataLength,
			&tbl.IndexLength,
			&tbl.DataFree,
			&tbl.AutoIncrement,
			&tbl.CreateTime,
			&tbl.UpdateTime,
			&tbl.CheckTime,
			&tbl.Collation,
			&tbl.Checksum,
			&tbl.CreateOptions,
			&tbl.Comment,
		)
		if err != nil {
			panic("Scan tables" + err.Error())
		}

		out = append(out, tbl)
	}

	return out, nil

}

// Field Field structure
type Field struct {
	ORDINAL_POSITION string  `json:"ORDINAL_POSITION"`
	COLUMN_COMMENT   string  `json:"COLUMN_COMMENT"`
	COLUMN_NAME      string  `json:"COLUMN_NAME"`
	COLUMN_DEFAULT   *string `json:"COLUMN_DEFAULT"`
	IS_NULLABLE      string  `json:"IS_NULLABLE"`
	DATA_TYPE        string  `json:"DATA_TYPE"`
	COLUMN_TYPE      string  `json:"COLUMN_TYPE"`
	EXTRA            string  `json:"EXTRA"`
	COLUMN_KEY       string  `json:"COLUMN_KEY"`
}

// GetFields get list of tables in a DB
func (d *DBConn) GetFields(table string) ([]Field, error) {
	result, err := d.Conn.Query(fmt.Sprintf(`SELECT ORDINAL_POSITION, COLUMN_COMMENT, `+"`COLUMN_NAME`"+`, COLUMN_DEFAULT, 
	IS_NULLABLE, DATA_TYPE, COLUMN_TYPE, EXTRA, COLUMN_KEY FROM INFORMATION_SCHEMA.COLUMNS
	WHERE TABLE_NAME = '%s%s' AND TABLE_SCHEMA = '%s'`, d.Prefix, table, d.Scheme))

	var out []Field
	if err != nil {
		return out, err
	}

	for result.Next() {
		var fld = Field{}
		err = result.Scan(
			&fld.ORDINAL_POSITION,
			&fld.COLUMN_COMMENT,
			&fld.COLUMN_NAME,
			&fld.COLUMN_DEFAULT,
			&fld.IS_NULLABLE,
			&fld.DATA_TYPE,
			&fld.COLUMN_TYPE,
			&fld.EXTRA,
			&fld.COLUMN_KEY,
		)
		if err != nil {
			panic("Scan tables" + err.Error())
		}

		out = append(out, fld)
	}

	return out, nil

}

// Index Index structure
type Index struct {
	Table         string  `json:"Table"`
	Non_unique    string  `json:"Non_unique"`
	Key_name      string  `json:"Key_name"`
	Seq_in_index  string  `json:"Seq_in_index"`
	Column_name   string  `json:"Column_name"`
	Collation     *string `json:"Collation"`
	Cardinality   *string `json:"Cardinality"`
	Sub_part      *string `json:"Sub_part"`
	Packed        *string `json:"Packed"`
	Null          string  `json:"Null"`
	Index_type    string  `json:"Index_type"`
	Comment       string  `json:"Comment"`
	Index_comment string  `json:"Index_comment"`
}

// GetIndexes get list of tables in a DB
func (d *DBConn) GetIndexes(table string) ([]Index, error) {
	result, err := d.Conn.Query(fmt.Sprintf(`SHOW INDEXES FROM %s%s`, d.Prefix, table))

	var out []Index
	if err != nil {
		return out, err
	}

	for result.Next() {
		var fld = Index{}
		err = result.Scan(
			&fld.Table,
			&fld.Non_unique,
			&fld.Key_name,
			&fld.Seq_in_index,
			&fld.Column_name,
			&fld.Collation,
			&fld.Cardinality,
			&fld.Sub_part,
			&fld.Packed,
			&fld.Null,
			&fld.Index_type,
			&fld.Comment,
			&fld.Index_comment,
		)
		if err != nil {
			panic("Scan tables" + err.Error())
		}

		out = append(out, fld)
	}

	return out, nil

}

// Constrain Constrain structure
type Constrain struct {
	UPDATE_RULE            string `json:"UPDATE_RULE"`
	DELETE_RULE            string `json:"DELETE_RULE"`
	CONSTRAINT_NAME        string `json:"CONSTRAINT_NAME"`
	COLUMN_NAME            string `json:"COLUMN_NAME"`
	REFERENCED_TABLE_NAME  string `json:"REFERENCED_TABLE_NAME"`
	REFERENCED_COLUMN_NAME string `json:"REFERENCED_COLUMN_NAME"`
}

// GetConstraines get list of tables in a DB
func (d *DBConn) GetConstraines(table string) ([]Constrain, error) {
	result, err := d.Conn.Query(fmt.Sprintf(`SELECT rc.UPDATE_RULE, rc.DELETE_RULE, 
	kc.CONSTRAINT_NAME,	kc.COLUMN_NAME, kc.REFERENCED_TABLE_NAME, kc.REFERENCED_COLUMN_NAME 
	FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE  kc
	LEFT JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS rc ON rc.CONSTRAINT_NAME = kc.CONSTRAINT_NAME
	WHERE kc.TABLE_NAME = '%s%s'
	AND kc.CONSTRAINT_NAME <> 'PRIMARY'
	AND rc.CONSTRAINT_SCHEMA  = '%s'
	AND kc.TABLE_SCHEMA = '%s'`, d.Prefix, table, d.Scheme, d.Scheme))

	var out []Constrain
	if err != nil {
		return out, err
	}

	for result.Next() {
		var fld = Constrain{}
		err = result.Scan(
			&fld.UPDATE_RULE,
			&fld.DELETE_RULE,
			&fld.CONSTRAINT_NAME,
			&fld.COLUMN_NAME,
			&fld.REFERENCED_TABLE_NAME,
			&fld.REFERENCED_COLUMN_NAME,
		)
		if err != nil {
			panic("Scan tables" + err.Error())
		}

		out = append(out, fld)
	}

	return out, nil

}

func Struct2json(s interface{}) map[string]interface{} {
	rtable := reflect.ValueOf(s)
	ltable := make(map[string]interface{}, rtable.NumField())
	types := rtable.Type()
	for i := 0; i < rtable.NumField(); i++ {
		// switch v := reflect.Indirect(rtable).FieldByName(types.Field(i).Name).Type(); v {
		// case v.String() == "":
		// default:
		// 	ltable[types.Field(i).Name] = v
		// }
		ltable[types.Field(i).Name] = rtable.Field(i).Interface()

	}
	// fmt.Println(ltable)
	return ltable
}
