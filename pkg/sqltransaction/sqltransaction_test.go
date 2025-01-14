package sqltransaction

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/nexmoinc/gosrvlib/pkg/testutil"
	"github.com/stretchr/testify/require"
)

func Test_Exec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupMocks func(mock sqlmock.Sqlmock)
		run        func(ctx context.Context, tx *sql.Tx) error
		wantErr    bool
	}{
		{
			name: "success",
			setupMocks: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectCommit()
			},
			run: func(ctx context.Context, tx *sql.Tx) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "rollback transaction",
			setupMocks: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback()
			},
			run: func(ctx context.Context, tx *sql.Tx) error {
				return fmt.Errorf("db error")
			},
			wantErr: true,
		},
		{
			name: "begin error",
			setupMocks: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(fmt.Errorf("begin error"))
			},
			run: func(ctx context.Context, tx *sql.Tx) error {
				return nil
			},
			wantErr: true,
		},
		{
			name: "commit error",
			setupMocks: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectCommit().WillReturnError(fmt.Errorf("commit error"))
			},
			run: func(ctx context.Context, tx *sql.Tx) error {
				return nil
			},
			wantErr: true,
		},
		{
			name: "rollback error",
			setupMocks: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback().WillReturnError(fmt.Errorf("rollback error"))
			},
			run: func(ctx context.Context, tx *sql.Tx) error {
				return fmt.Errorf("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer func() { _ = mockDB.Close() }()

			if tt.setupMocks != nil {
				tt.setupMocks(mock)
			}

			err = Exec(testutil.Context(), mockDB, tt.run)
			require.Equal(t, tt.wantErr, err != nil, "Exec() error = %v, wantErr %v", err, tt.wantErr)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
