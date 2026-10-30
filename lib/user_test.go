package lib

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestCheckPasswordRemembersOnlyAccepted(t *testing.T) {
	t.Parallel()

	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	require.NoError(t, err)

	u := &handlerUser{User: User{Password: "{bcrypt}" + string(hash)}}

	require.False(t, u.checkPassword("wrong"))
	require.Nil(t, u.verified.Load())

	require.True(t, u.checkPassword("secret"))
	require.NotNil(t, u.verified.Load())

	require.False(t, u.checkPassword("wrong"))
	require.True(t, u.checkPassword("secret"))
}
