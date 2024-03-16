# ToDos

1. Improve error handling
2. Write tests
3. Fix API spec

## API rework

GET   sources                                                           => show all type of sources

GET   sources/git-repo/${repoURL}                                       => show all paths with helm-charts and kustomizations
GET   sources/git-repo/${repoURL}/charts                                => show all paths with helm-charts
GET   sources/git-repo/${repoURL}/charts/${path}                        => show helm-chart representation (Chart.yaml, value files)
GET   sources/git-repo/${repoURL}/charts/${path}/defaultValues          => show default values

GET   sources/git-repo/${repoURL}/kustomizations                        => show all paths with kustomizations
GET   sources/git-repo/${repoURL}/kustomizations/${path}                => show kustomization representation (kustomization.yaml)

GET   sources/chart-repo/${repoURL}                                     => ???
GET   sources/chart-repo/${repoURL}/charts                              => show index representation
GET   sources/chart-repo/${repoURL}/charts/${name}                      => show helm-chart representation (Chart.yaml, value files)
GET   sources/chart-repo/${repoURL}/charts/${name}/defaultValues        => show default values
