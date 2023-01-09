import { spawn } from "child_process";

class SubConverter {
  async start() {
    await new Promise(() => {
      spawn("./bin/subconverter/subconverter").stderr.on("data", (data) => {
        // console.log(data.toString());
      });
    });
  }
}

const subconverter = new SubConverter();
export { subconverter };
