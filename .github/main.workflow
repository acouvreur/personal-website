workflow "Build and Publish" {
    on       = "push"

    resolves = [

    ]
}

action "Build" {
    uses = "actions/npm@master"
    args = "install"
}

action "Docker Login" {
    uses = "actions/docker/login@master"
    secrets = ["DOCKER_USERNAME", "DOCKER_PASSWORD"]
}

action "Docker Build" {
    needs = "Docker Login"
    uses = "actions/docker/cli@master"
    args = "build -t acouvreur/my-website:latest ."
}

action "Docker Publish" {
    needs   = "Docker Build"
    uses    = "actions/docker/cli@master"
    args    = "push acouvreur/my-website:latest"
}
