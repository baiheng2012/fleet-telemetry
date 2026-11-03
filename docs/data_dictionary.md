<!-- GENERATED from contracts/vehicle_semantics.yaml. DO NOT EDIT. -->
# Vehicle data dictionary

| Signal | Type | Unit | Valid range | Applies to |
|---|---|---|---|---|
| `engine_rpm` | float | rpm | [0, 3000] | LHD, Truck, Bolter |
| `coolant_temp` | float | celsius | [-20, 130] | LHD, Truck, Bolter |
| `hydraulic_pressure` | float | bar | [0, 350] | LHD, Truck, Bolter |
| `fuel_level` | float | percent | [0, 100] | LHD, Truck, Bolter |
| `hour_meter` | float | hour | [0, 100000] | LHD, Truck, Bolter |

Valid fault codes: `E101`, `E205`, `E310`, `E412`
