
CC_FLAGS += -I./inc

all: ved

ved: src/main.o
	$(LINK.cc) $^ $(LOADLIBES) $(LDLIBS) -o $@


