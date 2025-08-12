package bdd_test

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/bdd"
	"github.com/opendatahub-io/opendatahub-operator/v2/tests/envtestutil"
)

var (
	opts = godog.Options{
		Output:        colors.Colored(os.Stdout),
		Format:        "progress",
		Paths:         []string{"features"},
		Randomize:     0,
		Concurrency:   0,
		StopOnFailure: true,
	}
)

//nolint:gochecknoinits
func init() {
	log.SetLogger(zap.New(zap.UseDevMode(true)))
	godog.BindCommandLineFlags("godog.", &opts)
}

func TestFeatures(t *testing.T) {

	cfg, err := config.GetConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	root, err := envtestutil.FindProjectRoot()
	require.NoError(t, err)
	require.NotEmpty(t, root)

	// Run feature tests
	for _, path := range opts.Paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		featureFiles := set.New[string]()

		// Find feature files
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			if strings.HasSuffix(filePath, ".feature") {
				featureFiles.Insert(filePath)
			}

			return nil
		})

		if err != nil {
			t.Errorf("Failed to find feature files in %s: %v", path, err)
			continue
		}

		for _, featureFile := range featureFiles.SortedList() {
			name := strings.TrimPrefix(featureFile, root)
			name = strings.TrimSuffix(featureFile, ".feature")
			name = strings.ReplaceAll(name, "/", "_")

			t.Run(name, func(t *testing.T) {
				ctx := context.Background()

				testCtx, err := bdd.NewTestContext(cfg)
				require.NoError(t, err)

				opt := opts
				opt.TestingT = t
				opt.DefaultContext = ctx

				// Create test suite
				ts := godog.TestSuite{
					Name: name,
					TestSuiteInitializer: func(tsCtx *godog.TestSuiteContext) {
						/*
							tsCtx.AfterSuite(func() {
								ctx := context.Background()

								c := bdd.CapturingT{}
								g := c.NewWithT(testCtx.Config())

								g.Eventually(func() (any, error) {
									if err := testCtx.Client().DeleteAllOf(
										ctx,
										&dscv1.DataScienceCluster{},
										client.PropagationPolicy(metav1.DeletePropagationForeground),
									); err != nil {
										return nil, err
									}

									items := dscv1.DataScienceClusterList{}
									if err := testCtx.Client().List(ctx, &items); err != nil {
										return nil, err
									}

									return items.Items, nil
								}).Should(
									gomega.BeEmpty(),
								)

								if err := c.Error(); err != nil {
									t.Error(err)
								}

								g.Eventually(func() (any, error) {
									if err := testCtx.Client().DeleteAllOf(
										ctx,
										&dsciv1.DSCInitialization{},
										client.PropagationPolicy(metav1.DeletePropagationForeground),
									); err != nil {
										return 0, err
									}

									items := dsciv1.DSCInitializationList{}
									if err := testCtx.Client().List(ctx, &items); err != nil {
										return 0, err
									}

									return items.Items, nil
								}).Should(
									gomega.BeEmpty(),
								)

								if err := c.Error(); err != nil {
									t.Error(err)
								}
							})
						*/
					},
					ScenarioInitializer: func(sctx *godog.ScenarioContext) {
						sctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
							return context.WithValue(ctx, bdd.TestCtxKey{}, testCtx), nil
						})

						bdd.InitializeConfigurationSteps(sctx)
						bdd.InitializeVariableSteps(sctx)
						bdd.InitializeResourceSteps(sctx)
						bdd.InitializeComponentSteps(sctx)
					},
					Options: &opt,
				}

				// Run the test
				if ts.Run() != 0 {
					t.Error("BDD test suite failed")
				}
			})
		}
	}
}

func TestMain(m *testing.M) {
	for _, arg := range os.Args[1:] {
		if arg == "-test.v=true" || arg == "-test.v" || arg == "-v" {
			opts.Format = "pretty"
		}
	}

	if !flag.Parsed() {
		flag.Parse()
	}

	exitCode := m.Run()

	os.Exit(exitCode)
}
