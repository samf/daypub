package main

import (
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kong"
	"github.com/gomarkdown/markdown"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type Context struct{}

type CLI struct {
	Version kong.VersionFlag `help:"print current version and exit"`

	Markdown MarkdownCmd `help:"publish as simple markdown" cmd:""`
	Hugo     HugoCmd     `help:"publish as hugo content" cmd:""`
}

type CommonConvert struct {
	Input *os.File `help:"zip file to be converted" arg:""`

	OutDir     string   `help:"directory for output files; will create if necessary" arg:"" type:"path"`
	Entry      int      `help:"which entry to export" default:"0"`
	FileName   string   `help:"file name of the markdown file we will produce" default:"index.md"`
	PhotoNames []string `help:"friendly names for photos"`
}

type MarkdownCmd struct {
	CommonConvert
}

func (mdc *CommonConvert) getStuff(ctx *Context) (
	*DOZip,
	*Journal,
	*Entry,
	*PhotoWallet,
	*Body,
	error,
) {
	doz, err := mdc.getDOZip()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	journal, err := doz.getJournal()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	entry, err := journal.getEntry(mdc.Entry)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	photoWallet := NewPhotoWallet(entry)
	err = photoWallet.setFriendlyNames(mdc.PhotoNames)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	body := NewBody(entry)
	body.fixImages(photoWallet)

	return doz, journal, entry, photoWallet, body, nil
}

func (mdc *MarkdownCmd) Run(ctx *Context) error {
	defer mdc.Input.Close()

	doz, _, entry, _, body, err := mdc.getStuff(ctx)

	err = mdc.GotoOutDir()
	if err != nil {
		return err
	}

	err = mdc.outBody(body, nil, nil)
	if err != nil {
		return err
	}

	err = mdc.outPhotos(doz, entry)
	if err != nil {
		return err
	}

	return nil
}

func (cli *CommonConvert) GotoOutDir() error {
	err := os.MkdirAll(cli.OutDir, 0777)
	if err != nil {
		err = fmt.Errorf("cannot make directory %q: %w",
			cli.OutDir,
			err,
		)
		return err
	}
	err = os.Chdir(cli.OutDir)
	if err != nil {
		err = fmt.Errorf("cannot access directory %q: %w",
			cli.OutDir,
			err,
		)
		return err
	}

	return nil
}

func (cli *CommonConvert) outBody(
	body *Body,
	frontMatter []byte,
	renderer markdown.Renderer,
) error {
	file, err := os.OpenFile(
		cli.FileName,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0666,
	)
	if err != nil {
		err = fmt.Errorf("cannot open %q: %w", cli.FileName, err)
		return err
	}
	defer file.Close()

	if frontMatter != nil {
		_, err = file.Write(frontMatter)
		if err != nil {
			err = fmt.Errorf("error writing front matter: %w", err)
			return err
		}
	}

	body.renderMarkdown(file, renderer)

	return nil
}

func (cli *CommonConvert) outPhotos(doz *DOZip, entry *Entry) error {
	for _, p := range entry.Photos {
		zf, err := doz.findPhotoFile(p.MD5)
		if err != nil {
			return err
		}
		infile, err := zf.Open()
		if err != nil {
			err = fmt.Errorf("cannot open zip part for %q: %w",
				p.MD5,
				err,
			)
			return err
		}
		defer infile.Close()

		outname := p.getFName()
		outfile, err := os.OpenFile(
			outname,
			os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
			0666,
		)
		if err != nil {
			err = fmt.Errorf("cannot open output for photo %q: %w",
				outname,
				err,
			)
			return err
		}
		defer outfile.Close()

		_, err = io.Copy(outfile, infile)
		if err != nil {
			err = fmt.Errorf("problem writing image file %q: %w",
				outname,
				err,
			)
			return err
		}
	}

	return nil
}

func (cli *CommonConvert) getDOZip() (*DOZip, error) {
	doz, err := NewDOZip(cli.Input)
	if err != nil {
		return nil, err
	}

	return doz, nil
}

func main() {
	ctx := kong.Parse(&CLI{}, kong.Vars{
		"version": version,
		"commit":  commit,
		"date":    date,
	})
	err := ctx.Run(&Context{})
	ctx.FatalIfErrorf(err)
}
