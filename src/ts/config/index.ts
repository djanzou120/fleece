function get(key: string): string {
    key = key.replace(/[-.]/g, "_").toUpperCase()
  
    const val = process.env[key]
    if (val === undefined) {
      throw new Error(`missing key ${key}`)
    }
  
    return val
  }
  
  export { get }