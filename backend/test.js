const { execSync } = require("child_process");

try {
  // 1. Get the full list of packages (mirroring 'go list ./...')
  const allPackages = execSync("go list ./...", { encoding: "utf8" })
    .split(/\r?\n/)
    .filter((pkg) => pkg.trim() !== "");

  // 2. Filter out the folders you want to skip (mirroring 'grep -v')
  const filteredPackages = allPackages.filter((pkg) => {
    const isExcluded =
      pkg.includes("pkg/boards") || pkg.includes("pkg/broker/topics/message");
    return !isExcluded;
  });

  if (filteredPackages.length === 0) {
    console.log("No packages found to test.");
    process.exit(0);
  }

  // 3. Run the test command with the clean list
  const cmd = `go test -v -count=1 -timeout=20s ${filteredPackages.join(" ")}`;
  console.log(`Running: ${cmd}\n`);

  execSync(cmd, { stdio: "inherit" });
} catch (error) {
  // Exit with the same error code if tests fail
  process.exit(1);
}
