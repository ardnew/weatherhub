package network

type AP struct {
	SSID, Pass string
}

var Network = []AP{
	{
		SSID: "Shultzabarger",
		Pass: "Samus01!",
	},
}
