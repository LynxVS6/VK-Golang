package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
)

type M map[string]interface{}

// Column описывает столбец таблицы
type Column struct {
	Name            string
	DBType          string
	Kind            string // "int" | "float" | "string"
	Nullable        bool
	IsPK            bool
	IsAutoIncrement bool
}

// Table описывает таблицу
type Table struct {
	Name    string
	Columns []*Column
	ColMap  map[string]*Column
	PK      *Column
}

// DBExplorer — основной http.Handler
type DBExplorer struct {
	db         *sql.DB
	tables     map[string]*Table
	tableNames []string
}

func NewDBExplorer(db *sql.DB) (http.Handler, error) {
	// тесты ожидают максимум 1 открытое соединение
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ex := &DBExplorer{
		db:         db,
		tables:     make(map[string]*Table),
		tableNames: make([]string, 0),
	}

	// 1. читаем список таблиц
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// 2. читаем описание каждой таблицы
	for _, name := range tableNames {
		t, err := ex.loadTable(name)
		if err != nil {
			return nil, err
		}
		ex.tables[name] = t
		ex.tableNames = append(ex.tableNames, name)
	}

	return ex, nil
}

func (e *DBExplorer) loadTable(name string) (*Table, error) {
	query := fmt.Sprintf("SHOW FULL COLUMNS FROM `%s`", name)
	rows, err := e.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	t := &Table{
		Name:    name,
		Columns: make([]*Column, 0),
		ColMap:  make(map[string]*Column),
	}

	for rows.Next() {
		var field, colType, collation, isNull, key, defVal, extra, privileges, comment sql.NullString
		if err := rows.Scan(&field, &colType, &collation, &isNull, &key, &defVal, &extra, &privileges, &comment); err != nil {
			return nil, err
		}

		col := &Column{
			Name:            field.String,
			DBType:          colType.String,
			Kind:            mapDBType(colType.String),
			Nullable:        strings.ToUpper(isNull.String) == "YES",
			IsPK:            strings.ToUpper(key.String) == "PRI",
			IsAutoIncrement: strings.Contains(strings.ToLower(extra.String), "auto_increment"),
		}

		t.Columns = append(t.Columns, col)
		t.ColMap[col.Name] = col
		if col.IsPK {
			t.PK = col
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return t, nil
}

func mapDBType(s string) string {
	s = strings.ToLower(s)
	switch {
	case strings.Contains(s, "int"):
		return "int"
	case strings.Contains(s, "float") || strings.Contains(s, "double") || strings.Contains(s, "decimal"):
		return "float"
	default:
		return "string"
	}
}

func (e *DBExplorer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := r.URL.Path
	if path == "/" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "bad method")
			return
		}
		writeJSON(w, http.StatusOK, M{"response": M{"tables": e.tableNames}})
		return
	}

	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		writeError(w, http.StatusNotFound, "unknown table")
		return
	}

	parts := strings.Split(trimmed, "/")
	tableName := parts[0]

	table, ok := e.tables[tableName]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown table")
		return
	}

	var idStr string
	if len(parts) > 1 {
		idStr = parts[1]
	}

	switch r.Method {
	case http.MethodGet:
		if idStr == "" {
			e.handleList(w, r, table)
		} else {
			e.handleGet(w, r, table, idStr)
		}
	case http.MethodPut:
		if idStr != "" {
			writeError(w, http.StatusNotFound, "unknown table")
			return
		}
		e.handleCreate(w, r, table)
	case http.MethodPost:
		if idStr == "" {
			writeError(w, http.StatusNotFound, "record not found")
			return
		}
		e.handleUpdate(w, r, table, idStr)
	case http.MethodDelete:
		if idStr == "" {
			writeError(w, http.StatusNotFound, "record not found")
			return
		}
		e.handleDelete(w, r, table, idStr)
	default:
		writeError(w, http.StatusMethodNotAllowed, "bad method")
	}
}

// ---------- handlers ----------

func (e *DBExplorer) handleList(w http.ResponseWriter, r *http.Request, table *Table) {
	q := r.URL.Query()
	limit := parseIntDefault(q.Get("limit"), 5)
	offset := parseIntDefault(q.Get("offset"), 0)

	var query string
	if table.PK != nil {
		query = fmt.Sprintf("SELECT * FROM `%s` ORDER BY `%s` LIMIT %d OFFSET %d",
			table.Name, table.PK.Name, limit, offset)
	} else {
		query = fmt.Sprintf("SELECT * FROM `%s` LIMIT %d OFFSET %d",
			table.Name, limit, offset)
	}

	rows, err := e.db.Query(query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	records, err := scanRows(rows, table)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, M{"response": M{"records": records}})
}

func (e *DBExplorer) handleGet(w http.ResponseWriter, r *http.Request, table *Table, idStr string) {
	if table.PK == nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusNotFound, "record not found")
		return
	}

	query := fmt.Sprintf("SELECT * FROM `%s` WHERE `%s` = ? LIMIT 1",
		table.Name, table.PK.Name)
	rows, err := e.db.Query(query, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	records, err := scanRows(rows, table)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if len(records) == 0 {
		writeError(w, http.StatusNotFound, "record not found")
		return
	}

	writeJSON(w, http.StatusOK, M{"response": M{"record": records[0]}})
}

func (e *DBExplorer) handleCreate(w http.ResponseWriter, r *http.Request, table *Table) {
	input, ok := decodeJSON(w, r)
	if !ok {
		return
	}

	cols := make([]string, 0)
	vals := make([]interface{}, 0)
	ph := make([]string, 0)

	for _, col := range table.Columns {
		if col.IsPK && col.IsAutoIncrement {
			continue
		}

		raw, exists := input[col.Name]

		var (
			val   interface{}
			valid bool
		)

		if exists {
			val, valid = convertValue(col, raw)
			if !valid {
				writeError(w, http.StatusBadRequest,
					fmt.Sprintf("field %s have invalid type", col.Name))
				return
			}
		} else {
			if col.Nullable {
				val = nil
			} else {
				switch col.Kind {
				case "int":
					val = int64(0)
				case "float":
					val = float64(0)
				default: // string
					val = ""
				}
			}
		}

		cols = append(cols, fmt.Sprintf("`%s`", col.Name))
		ph = append(ph, "?")
		vals = append(vals, val)
	}

	var query string
	if len(cols) == 0 {
		query = fmt.Sprintf("INSERT INTO `%s` () VALUES ()", table.Name)
	} else {
		query = fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)",
			table.Name, strings.Join(cols, ", "), strings.Join(ph, ", "))
	}

	res, err := e.db.Exec(query, vals...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := M{}
	if table.PK != nil {
		if table.PK.IsAutoIncrement {
			id, err := res.LastInsertId()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			resp[table.PK.Name] = id
		} else if v, ok := input[table.PK.Name]; ok {
			resp[table.PK.Name] = v
		}
	}

	writeJSON(w, http.StatusOK, M{"response": resp})
}

func (e *DBExplorer) handleUpdate(w http.ResponseWriter, r *http.Request, table *Table, idStr string) {
	if table.PK == nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusNotFound, "record not found")
		return
	}

	input, ok := decodeJSON(w, r)
	if !ok {
		return
	}

	sets := make([]string, 0)
	vals := make([]interface{}, 0)

	for name, raw := range input {
		col, ok := table.ColMap[name]
		if !ok {
			continue
		}
		if col.IsPK {
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("field %s have invalid type", col.Name))
			return
		}

		val, valid := convertValue(col, raw)
		if !valid {
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("field %s have invalid type", col.Name))
			return
		}

		sets = append(sets, fmt.Sprintf("`%s` = ?", col.Name))
		vals = append(vals, val)
	}

	if len(sets) == 0 {
		writeJSON(w, http.StatusOK, M{"response": M{"updated": 0}})
		return
	}

	vals = append(vals, id)
	query := fmt.Sprintf("UPDATE `%s` SET %s WHERE `%s` = ?",
		table.Name, strings.Join(sets, ", "), table.PK.Name)

	res, err := e.db.Exec(query, vals...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	aff, err := res.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, M{"response": M{"updated": aff}})
}

func (e *DBExplorer) handleDelete(w http.ResponseWriter, r *http.Request, table *Table, idStr string) {
	if table.PK == nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusNotFound, "record not found")
		return
	}

	query := fmt.Sprintf("DELETE FROM `%s` WHERE `%s` = ?",
		table.Name, table.PK.Name)
	res, err := e.db.Exec(query, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	aff, err := res.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, M{"response": M{"deleted": aff}})
}

// ---------- утилиты ----------

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return def
	}
	return v
}

func decodeJSON(w http.ResponseWriter, r *http.Request) (map[string]interface{}, bool) {
	var m map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return nil, false
	}
	return m, true
}

func convertValue(col *Column, v interface{}) (interface{}, bool) {
	if v == nil {
		if !col.Nullable {
			return nil, false
		}
		return nil, true
	}

	switch col.Kind {
	case "int":
		num, ok := v.(float64)
		if !ok || math.IsNaN(num) || math.IsInf(num, 0) {
			return nil, false
		}
		return int64(num), true
	case "float":
		num, ok := v.(float64)
		if !ok || math.IsNaN(num) || math.IsInf(num, 0) {
			return nil, false
		}
		return num, true
	default: // string
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		return s, true
	}
}

// scanRows читает все строки в []map[columnName]value, учитывая типы колонок
func scanRows(rows *sql.Rows, table *Table) ([]M, error) {
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	res := make([]M, 0)
	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		row := make(M, len(cols))
		for i, name := range cols {
			val := values[i]
			if val == nil {
				row[name] = nil
				continue
			}

			if b, ok := val.([]byte); ok {
				col, hasCol := table.ColMap[name]
				s := string(b)

				if !hasCol {
					row[name] = s
					continue
				}

				switch col.Kind {
				case "int":
					n, err := strconv.ParseInt(s, 10, 64)
					if err != nil {
						row[name] = s
					} else {
						row[name] = n
					}
				case "float":
					f, err := strconv.ParseFloat(s, 64)
					if err != nil {
						row[name] = s
					} else {
						row[name] = f
					}
				default:
					row[name] = s
				}
			} else {
				row[name] = val
			}
		}
		res = append(res, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, M{"error": msg})
}
