//go:build !embedadmin

package main

import "github.com/go-chi/chi/v5"

func registerAdminUI(_ chi.Router) {}
