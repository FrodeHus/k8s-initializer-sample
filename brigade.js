const { events, Job } = require("brigadier");

events.on("exec", function(e, project) {
  console.log("received push for commit " + e.commit)

  // Create a new job
  var node = new Job("build-go")

  // We want our job to run the stock Docker Python 3 image
  node.image = "mcuadros/golang-arm:1.9-alpine"

  // Now we want it to run these commands in order:
  node.tasks = [
    "cd /src",
    "dep ensure",
    "go build"
  ]

  // We're done configuring, so we run the job
  node.run()
})
