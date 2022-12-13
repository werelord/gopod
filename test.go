package main

import (
	"gopod/pod"
)

//lint:file-ignore U1000 Ignore all unused methods, one-time or testing use only

func doMigrate(feedmap map[string]*pod.Feed, db *pod.PodDB) {

	//db.DoMigrateFeed()

}

/*
func testBadHelp(args []string) {

	opt := getoptions.New()
	opt.SetUnknownMode(getoptions.Pass)

	var config string
	opt.StringVar(&config, "config", "",
		opt.Required("config required"),
		opt.Description("config file"),
	)

	testCommand := opt.NewCommand("foo", "foobar")
	testCommand.SetCommandFn(func(context.Context, *getoptions.GetOpt, []string) error {
		fmt.Print("testcommand")
		return nil
	})

	opt.HelpCommand("help", opt.Alias("h", "?"))

	remaining, err := opt.Parse(args[1:])
	if err != nil {
		fmt.Printf("parse error: %v", err)
		return
	}

	ctx, cancel, done := getoptions.InterruptContext()
	defer func() { cancel(); <-done }()

	if err := opt.Dispatch(ctx, remaining); err != nil {
		if errors.Is(err, getoptions.ErrorHelpCalled) {
			fmt.Print("help called\n")
		} else {
			fmt.Printf("dispatch err: %v", err)
		}
		return
	}

	fmt.Printf("config: %v\n", config)

}
*/
